package admin

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/kelongyan/ModelMux/config"
	"github.com/kelongyan/ModelMux/logx"
	"github.com/kelongyan/ModelMux/pool"
	"github.com/kelongyan/ModelMux/proxy"
)

func (h *Handler) updateProviderKeyMetadata(w http.ResponseWriter, r *http.Request, id, keyID string) {
	if r.Method != http.MethodPatch {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if !h.requireConfigManager(w) {
		return
	}
	if keyID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "key_id is required"})
		return
	}

	var req apiKeyMetadataPayload
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.Label == nil && req.Note == nil && req.Disabled == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "at least one metadata field is required"})
		return
	}

	result, err := h.cfgManager.Update(func(cfg *config.Config) error {
		idx := findProviderIndex(cfg.Providers, id)
		if idx < 0 {
			return fmt.Errorf("provider not found")
		}
		provider := &cfg.Providers[idx]
		if _, ok := findProviderKeyValue(*provider, keyID); !ok {
			return fmt.Errorf("key not found")
		}

		metadata, _ := provider.KeyMetadataForID(keyID)
		if req.Label != nil {
			metadata.Label = strings.TrimSpace(*req.Label)
		}
		if req.Note != nil {
			metadata.Note = strings.TrimSpace(*req.Note)
		}
		if req.Disabled != nil {
			metadata.Disabled = *req.Disabled
		}

		if keyMetadataEmptyAdmin(metadata) {
			if provider.KeyMetadata != nil {
				delete(provider.KeyMetadata, keyID)
				if len(provider.KeyMetadata) == 0 {
					provider.KeyMetadata = nil
				}
			}
			return nil
		}
		if provider.KeyMetadata == nil {
			provider.KeyMetadata = make(map[string]config.KeyMetadata, 1)
		}
		provider.KeyMetadata[keyID] = metadata
		return nil
	})
	if err != nil {
		h.recordEvent("error", logx.CategoryAdmin, "admin.key_metadata_failed", "key metadata update failed", map[string]any{
			"provider_id": id,
			"key_id":      keyID,
			"error":       err.Error(),
		})
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	h.recordEvent("info", logx.CategoryAdmin, "admin.key_metadata_updated", "key metadata updated", map[string]any{
		"provider_id": id,
		"key_id":      keyID,
	})
	writeJSON(w, http.StatusOK, h.toChangeResponse(result))
}

func (h *Handler) previewProviderKeys(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if !h.requireConfigManager(w) {
		return
	}

	var req apiKeysPreviewPayload
	if !decodeJSONBody(w, r, &req) {
		return
	}
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "append"
	}
	if mode != "append" && mode != "replace" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "mode must be append or replace"})
		return
	}
	normalized := normalizeKeys(req.Keys)
	if len(normalized) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "at least one key is required"})
		return
	}

	cfg, err := h.cfgManager.Snapshot()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}
	providerCfg, ok := findProviderConfig(cfg.ProviderConfigs(), id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "provider not found"})
		return
	}

	writeJSON(w, http.StatusOK, buildKeysPreviewResponse(providerCfg, mode, req.Keys, normalized))
}

func (h *Handler) resetAllProviderKeys(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}

	keyPool, err := h.pools.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	resetCount := keyPool.ResetAll()
	if resetCount > 0 {
		h.stateChanged(true)
	}
	h.recordEvent("info", logx.CategoryAdmin, "admin.keys_reset_all", "provider keys reset", map[string]any{
		"provider_id": id,
		"reset_count": resetCount,
	})
	writeJSON(w, http.StatusOK, apiKeysResetAllResponse{OK: true, ResetCount: resetCount})
}

func (h *Handler) testProviderKey(w http.ResponseWriter, r *http.Request, id, keyID string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if !h.requireConfigManager(w) {
		return
	}
	if keyID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "key_id is required"})
		return
	}

	cfg, err := h.cfgManager.Snapshot()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}
	providerCfg, ok := findProviderConfig(cfg.ProviderConfigs(), id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "provider not found"})
		return
	}
	keyValue, ok := findProviderKeyValue(providerCfg, keyID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "key not found"})
		return
	}

	result := proxy.ProbeKey(r.Context(), cfg, providerCfg, keyValue)
	level := "info"
	if !result.OK {
		level = "warn"
	}
	h.recordEvent(level, logx.CategoryAdmin, "admin.key_tested", "provider key tested", map[string]any{
		"provider_id": id,
		"key_id":      keyID,
		"ok":          result.OK,
		"status":      result.StatusCode,
		"scope":       result.Scope,
	})
	writeJSON(w, http.StatusOK, result)
}

func buildProviderKeyDetails(providerCfg config.ProviderConfig, statuses []pool.KeyStatus) []apiProviderKeyDetail {
	statusByID := make(map[string]pool.KeyStatus, len(statuses))
	for _, status := range statuses {
		statusByID[status.KeyID] = status
	}

	out := make([]apiProviderKeyDetail, 0, len(providerCfg.Keys))
	for _, keyValue := range providerCfg.Keys {
		keyID := poolKeyID(keyValue)
		metadata, _ := providerCfg.KeyMetadataForID(keyID)
		if metadata.Disabled {
			out = append(out, apiProviderKeyDetail{
				KeyID:     keyID,
				MaskedKey: logx.MaskSecret(keyValue),
				State:     "disabled",
				Label:     metadata.Label,
				Note:      metadata.Note,
				Disabled:  true,
			})
			continue
		}

		status, ok := statusByID[keyID]
		if !ok {
			status = pool.KeyStatus{
				KeyID:     keyID,
				MaskedKey: logx.MaskSecret(keyValue),
				State:     "active",
			}
		}
		out = append(out, apiProviderKeyDetail{
			KeyID:         status.KeyID,
			MaskedKey:     status.MaskedKey,
			State:         status.State,
			ReqCount:      status.ReqCount,
			ErrCount:      status.ErrCount,
			InFlight:      status.InFlight,
			AvgLatencyMs:  status.AvgLatencyMs,
			CoolUntil:     status.CoolUntil,
			Last401At:     status.Last401At,
			InvalidReason: status.InvalidReason,
			Label:         metadata.Label,
			Note:          metadata.Note,
			Disabled:      metadata.Disabled,
		})
	}
	return out
}

func buildKeysPreviewResponse(providerCfg config.ProviderConfig, mode string, rawKeys, normalizedKeys []string) apiKeysPreviewResponse {
	existingByID := make(map[string]string, len(providerCfg.Keys))
	for _, key := range providerCfg.Keys {
		existingByID[poolKeyID(key)] = key
	}

	resp := apiKeysPreviewResponse{
		Mode:            mode,
		InputCount:      len(rawKeys),
		NormalizedCount: len(normalizedKeys),
		DuplicateCount:  len(rawKeys) - len(normalizedKeys),
		ExistingKeys:    []apiKeyPreviewEntry{},
		NewKeys:         []apiKeyPreviewEntry{},
		RemovedKeys:     []apiKeyPreviewEntry{},
	}
	incomingByID := make(map[string]string, len(normalizedKeys))
	for _, key := range normalizedKeys {
		keyID := poolKeyID(key)
		incomingByID[keyID] = key
		if existingKey, ok := existingByID[keyID]; ok {
			resp.ExistingKeys = append(resp.ExistingKeys, previewEntry(providerCfg, existingKey))
			continue
		}
		resp.NewKeys = append(resp.NewKeys, previewEntry(providerCfg, key))
	}
	if mode == "replace" {
		for _, key := range providerCfg.Keys {
			if _, ok := incomingByID[poolKeyID(key)]; ok {
				continue
			}
			resp.RemovedKeys = append(resp.RemovedKeys, previewEntry(providerCfg, key))
		}
	}
	resp.ExistingCount = len(resp.ExistingKeys)
	resp.NewCount = len(resp.NewKeys)
	resp.RemovedCount = len(resp.RemovedKeys)
	return resp
}

func previewEntry(providerCfg config.ProviderConfig, keyValue string) apiKeyPreviewEntry {
	metadata, _ := providerCfg.KeyMetadataForValue(keyValue)
	return apiKeyPreviewEntry{
		KeyID:     poolKeyID(keyValue),
		MaskedKey: logx.MaskSecret(keyValue),
		Label:     metadata.Label,
		Disabled:  metadata.Disabled,
	}
}

func findProviderKeyValue(providerCfg config.ProviderConfig, keyID string) (string, bool) {
	for _, key := range providerCfg.Keys {
		if poolKeyID(key) == keyID {
			return key, true
		}
	}
	return "", false
}

func keyMetadataEmptyAdmin(metadata config.KeyMetadata) bool {
	return metadata.Label == "" && metadata.Note == "" && !metadata.Disabled
}

type apiKeyTestResult struct {
	KeyID     string `json:"key_id"`
	MaskedKey string `json:"masked_key"`
	OK        bool   `json:"ok"`
	Status    int    `json:"status_code,omitempty"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Scope     string `json:"scope,omitempty"`
	Error     string `json:"error,omitempty"`
}

type apiKeysTestAllResponse struct {
	OK      bool               `json:"ok"`
	Results []apiKeyTestResult `json:"results"`
}

// testAllProviderKeys 并发测试指定 provider 下所有启用的 key。
func (h *Handler) testAllProviderKeys(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if !h.requireConfigManager(w) {
		return
	}

	cfg, err := h.cfgManager.Snapshot()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}
	providerCfg, ok := findProviderConfig(cfg.ProviderConfigs(), id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "provider not found"})
		return
	}

	// 收集需要测试的 key
	type keyEntry struct {
		keyID     string
		keyValue  string
		maskedKey string
		disabled  bool
	}
	var entries []keyEntry
	for _, keyValue := range providerCfg.Keys {
		keyID := poolKeyID(keyValue)
		metadata, _ := providerCfg.KeyMetadataForID(keyID)
		entries = append(entries, keyEntry{
			keyID:     keyID,
			keyValue:  keyValue,
			maskedKey: logx.MaskSecret(keyValue),
			disabled:  metadata.Disabled,
		})
	}

	// 并发测试
	results := make([]apiKeyTestResult, len(entries))
	var wg sync.WaitGroup
	for i, entry := range entries {
		if entry.disabled {
			results[i] = apiKeyTestResult{
				KeyID:     entry.keyID,
				MaskedKey: entry.maskedKey,
				OK:        false,
				Error:     "key is disabled",
			}
			continue
		}
		wg.Add(1)
		go func(idx int, e keyEntry) {
			defer wg.Done()
			result := proxy.ProbeKey(r.Context(), cfg, providerCfg, e.keyValue)
			results[idx] = apiKeyTestResult{
				KeyID:     e.keyID,
				MaskedKey: e.maskedKey,
				OK:        result.OK,
				Status:    result.StatusCode,
				LatencyMs: result.LatencyMs,
				Scope:     result.Scope,
				Error:     result.Error,
			}
		}(i, entry)
	}
	wg.Wait()

	// 统计结果
	allOK := true
	for _, r := range results {
		if !r.OK {
			allOK = false
			break
		}
	}

	h.recordEvent("info", logx.CategoryAdmin, "admin.keys_test_all", "all provider keys tested", map[string]any{
		"provider_id": id,
		"total":       len(results),
		"ok":          allOK,
	})
	writeJSON(w, http.StatusOK, apiKeysTestAllResponse{OK: allOK, Results: results})
}

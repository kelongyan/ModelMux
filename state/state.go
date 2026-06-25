package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const CurrentVersion = 2

type KeyRecord struct {
	KeyID          string    `json:"key_id"`
	State          string    `json:"state"`
	CoolUntil      time.Time `json:"cool_until,omitempty"`
	ReqCount       int64     `json:"req_count"`
	ErrCount       int64     `json:"err_count"`
	TotalLatencyMs int64     `json:"total_latency_ms"`
	Last401At      time.Time `json:"last_401_at,omitempty"`
	InvalidReason  string    `json:"invalid_reason,omitempty"`
}

type ProviderRecord struct {
	ID   string      `json:"id"`
	Keys []KeyRecord `json:"keys"`
}

type File struct {
	Version   int              `json:"version"`
	SavedAt   time.Time        `json:"saved_at"`
	Keys      []KeyRecord      `json:"keys,omitempty"`
	Providers []ProviderRecord `json:"providers,omitempty"`
}

type Store struct {
	path string
	now  func() time.Time
}

// NewStore 创建基于 JSON 文件的状态仓库。
func NewStore(path string) *Store {
	return &Store{
		path: path,
		now:  time.Now,
	}
}

// Load 读取状态文件；文件不存在时返回空状态。
func (s *Store) Load() (*File, error) {
	// 清理可能残留的临时文件（上次写入未完成）
	tmpPath := s.path + ".tmp"
	_ = os.Remove(tmpPath)

	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return &File{Version: CurrentVersion}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}

	var file File
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	if file.Version != CurrentVersion && file.Version != 1 {
		return nil, fmt.Errorf("unsupported state version %d", file.Version)
	}
	if err := validateFile(&file); err != nil {
		return nil, err
	}
	return &file, nil
}

// validateFile 校验加载的状态文件中 key 状态枚举值是否合法。
func validateFile(f *File) error {
	validateKeys := func(keys []KeyRecord, prefix string) error {
		for i, k := range keys {
			switch k.State {
			case "active", "cooling", "invalid":
				// 合法状态
			default:
				return fmt.Errorf("%s[%d].state: invalid value %q, must be active, cooling, or invalid", prefix, i, k.State)
			}
		}
		return nil
	}
	if err := validateKeys(f.Keys, "keys"); err != nil {
		return err
	}
	for i, p := range f.Providers {
		if err := validateKeys(p.Keys, fmt.Sprintf("providers[%d].keys", i)); err != nil {
			return err
		}
	}
	return nil
}

// VersionedProviderRecords 返回向后兼容后的 provider 分组状态。
func (f *File) VersionedProviderRecords(activeProviderID string) []ProviderRecord {
	if len(f.Providers) > 0 {
		return append([]ProviderRecord(nil), f.Providers...)
	}
	if len(f.Keys) == 0 {
		return nil
	}
	id := activeProviderID
	if id == "" {
		id = "default"
	}
	return []ProviderRecord{{
		ID:   id,
		Keys: append([]KeyRecord(nil), f.Keys...),
	}}
}

// Save 原子写入按 provider 分组的状态文件，避免进程中断留下半截 JSON。
func (s *Store) Save(providers []ProviderRecord) error {
	file := File{
		Version:   CurrentVersion,
		SavedAt:   s.now(),
		Providers: providers,
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	data = append(data, '\n')

	if dir := filepath.Dir(s.path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create state directory: %w", err)
		}
	}

	tmpPath := s.path + ".tmp"
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("write state tmp: %w", err)
	}
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write state tmp: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("sync state tmp: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close state tmp: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		bakPath := s.path + ".bak"
		if renameErr := os.Rename(s.path, bakPath); renameErr != nil && !errors.Is(renameErr, os.ErrNotExist) {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("backup old state: %w", renameErr)
		}
		if renameErr := os.Rename(tmpPath, s.path); renameErr != nil {
			_ = os.Rename(bakPath, s.path)
			_ = os.Remove(tmpPath)
			return fmt.Errorf("replace state: %w", renameErr)
		}
		_ = os.Remove(bakPath)
	}
	return nil
}

// KeyID 计算 key 的稳定哈希标识，状态文件不保存完整 key。
func KeyID(key string) string {
	sum := sha256.Sum256([]byte(key))
	return "sha256:" + hex.EncodeToString(sum[:])
}

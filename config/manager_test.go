package config

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"
)

// TestReadDoesNotReplaceCurrent 验证 Read 只做预检查，不会污染当前运行时配置快照。
func TestReadDoesNotReplaceCurrent(t *testing.T) {
	firstPath := filepath.Join(t.TempDir(), "first.json")
	secondPath := filepath.Join(t.TempDir(), "second.json")

	first := mustNormalizedConfig(t, &Config{
		ActiveProvider: "p1",
		Providers: []ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
		},
	})
	second := mustNormalizedConfig(t, &Config{
		ActiveProvider: "p2",
		Providers: []ProviderConfig{
			{ID: "p2", TargetURL: "https://two.example.com", Keys: []string{"k2"}},
		},
	})
	if err := writeFileAtomic(firstPath, first); err != nil {
		t.Fatalf("writeFileAtomic(first) error = %v", err)
	}
	if err := writeFileAtomic(secondPath, second); err != nil {
		t.Fatalf("writeFileAtomic(second) error = %v", err)
	}

	if _, err := Load(firstPath); err != nil {
		t.Fatalf("Load(firstPath) error = %v", err)
	}
	if got := Get(); got == nil || got.ActiveProvider != "p1" {
		t.Fatalf("Get().ActiveProvider = %v, want p1", got)
	}

	preview, err := Read(secondPath)
	if err != nil {
		t.Fatalf("Read(secondPath) error = %v", err)
	}
	if preview.ActiveProvider != "p2" {
		t.Fatalf("preview.ActiveProvider = %q, want p2", preview.ActiveProvider)
	}
	if got := Get(); got == nil || got.ActiveProvider != "p1" {
		t.Fatalf("Get().ActiveProvider = %v, want p1 after Read", got)
	}
}

// TestManagerUpdateWritesAndReloads 验证配置管理器会原子写盘并在成功后提交新的 current。
func TestManagerUpdateWritesAndReloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	initial := mustNormalizedConfig(t, &Config{
		ActiveProvider: "p1",
		Providers: []ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
			{ID: "p2", TargetURL: "https://two.example.com", Keys: []string{"k2"}},
		},
	})
	if err := writeFileAtomic(path, initial); err != nil {
		t.Fatalf("writeFileAtomic() error = %v", err)
	}

	reloads := 0
	manager := NewManager(path, func(path string) error {
		reloads++
		cfg, err := Read(path)
		if err != nil {
			return err
		}
		SetCurrent(cfg)
		return nil
	})

	result, err := manager.Update(func(cfg *Config) error {
		cfg.ActiveProvider = "p2"
		cfg.CoolingSeconds = 99
		return nil
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if reloads != 1 {
		t.Fatalf("reloads = %d, want 1", reloads)
	}
	if result == nil || len(result.ChangedFields) == 0 {
		t.Fatalf("result = %#v, want changed fields", result)
	}

	loaded, err := Read(path)
	if err != nil {
		t.Fatalf("Read(path) error = %v", err)
	}
	if loaded.ActiveProvider != "p2" {
		t.Fatalf("loaded.ActiveProvider = %q, want p2", loaded.ActiveProvider)
	}
	if loaded.CoolingSeconds != 99 {
		t.Fatalf("loaded.CoolingSeconds = %d, want 99", loaded.CoolingSeconds)
	}
	if current := Get(); current == nil || current.ActiveProvider != "p2" {
		t.Fatalf("current.ActiveProvider = %#v, want p2", current)
	}
}

func TestManagerUpdateTreatsProviderCircuitFieldsAsHotReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	initial := mustNormalizedConfig(t, &Config{
		ActiveProvider: "p1",
		Providers: []ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
		},
	})
	if err := writeFileAtomic(path, initial); err != nil {
		t.Fatalf("writeFileAtomic() error = %v", err)
	}

	reloads := 0
	manager := NewManager(path, func(path string) error {
		reloads++
		cfg, err := Read(path)
		if err != nil {
			return err
		}
		SetCurrent(cfg)
		return nil
	})

	result, err := manager.Update(func(cfg *Config) error {
		cfg.ProviderCircuitFailureThreshold = 5
		cfg.ProviderCircuitOpenSeconds = 7
		cfg.ProviderCircuitMaxOpenSeconds = 42
		cfg.ProviderCircuitHalfOpenMax = 2
		return nil
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if reloads != 1 {
		t.Fatalf("reloads = %d, want 1", reloads)
	}
	wantFields := []string{
		"provider_circuit_failure_threshold",
		"provider_circuit_open_seconds",
		"provider_circuit_max_open_seconds",
		"provider_circuit_half_open_max",
	}
	if !slices.Equal(result.ChangedFields, wantFields) {
		t.Fatalf("ChangedFields = %#v, want %#v", result.ChangedFields, wantFields)
	}
	if !slices.Equal(result.HotReloadedFields, wantFields) {
		t.Fatalf("HotReloadedFields = %#v, want %#v", result.HotReloadedFields, wantFields)
	}
	if len(result.RestartRequiredFields) != 0 {
		t.Fatalf("RestartRequiredFields = %#v, want none", result.RestartRequiredFields)
	}

	loaded, err := Read(path)
	if err != nil {
		t.Fatalf("Read(path) error = %v", err)
	}
	if loaded.ProviderCircuitFailureThreshold != 5 {
		t.Fatalf("ProviderCircuitFailureThreshold = %d, want 5", loaded.ProviderCircuitFailureThreshold)
	}
	if loaded.ProviderCircuitOpenSeconds != 7 {
		t.Fatalf("ProviderCircuitOpenSeconds = %d, want 7", loaded.ProviderCircuitOpenSeconds)
	}
	if loaded.ProviderCircuitMaxOpenSeconds != 42 {
		t.Fatalf("ProviderCircuitMaxOpenSeconds = %d, want 42", loaded.ProviderCircuitMaxOpenSeconds)
	}
	if loaded.ProviderCircuitHalfOpenMax != 2 {
		t.Fatalf("ProviderCircuitHalfOpenMax = %d, want 2", loaded.ProviderCircuitHalfOpenMax)
	}
}

func TestManagerUpdateTreatsStreamFieldsAsHotReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	initial := mustNormalizedConfig(t, &Config{
		ActiveProvider: "p1",
		Providers: []ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
		},
	})
	if err := writeFileAtomic(path, initial); err != nil {
		t.Fatalf("writeFileAtomic() error = %v", err)
	}

	reloads := 0
	manager := NewManager(path, func(path string) error {
		reloads++
		cfg, err := Read(path)
		if err != nil {
			return err
		}
		SetCurrent(cfg)
		return nil
	})

	result, err := manager.Update(func(cfg *Config) error {
		cfg.StreamKeepAliveSeconds = 3
		cfg.StreamIdleTimeoutSeconds = 45
		cfg.StreamMaxDurationSeconds = 600
		return nil
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if reloads != 1 {
		t.Fatalf("reloads = %d, want 1", reloads)
	}
	wantFields := []string{
		"stream_keepalive_seconds",
		"stream_idle_timeout_seconds",
		"stream_max_duration_seconds",
	}
	if !slices.Equal(result.ChangedFields, wantFields) {
		t.Fatalf("ChangedFields = %#v, want %#v", result.ChangedFields, wantFields)
	}
	if !slices.Equal(result.HotReloadedFields, wantFields) {
		t.Fatalf("HotReloadedFields = %#v, want %#v", result.HotReloadedFields, wantFields)
	}
	if len(result.RestartRequiredFields) != 0 {
		t.Fatalf("RestartRequiredFields = %#v, want none", result.RestartRequiredFields)
	}

	loaded, err := Read(path)
	if err != nil {
		t.Fatalf("Read(path) error = %v", err)
	}
	if loaded.StreamKeepAliveSeconds != 3 {
		t.Fatalf("StreamKeepAliveSeconds = %d, want 3", loaded.StreamKeepAliveSeconds)
	}
	if loaded.StreamIdleTimeoutSeconds != 45 {
		t.Fatalf("StreamIdleTimeoutSeconds = %d, want 45", loaded.StreamIdleTimeoutSeconds)
	}
	if loaded.StreamMaxDurationSeconds != 600 {
		t.Fatalf("StreamMaxDurationSeconds = %d, want 600", loaded.StreamMaxDurationSeconds)
	}
}

// TestManagerUpdateRollsBackOnReloadError 验证 reload 失败时配置文件会自动回滚。
func TestManagerUpdateRollsBackOnReloadError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	initial := mustNormalizedConfig(t, &Config{
		ActiveProvider: "p1",
		Providers: []ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
		},
	})
	if err := writeFileAtomic(path, initial); err != nil {
		t.Fatalf("writeFileAtomic() error = %v", err)
	}

	reloads := 0
	manager := NewManager(path, func(path string) error {
		reloads++
		if reloads == 1 {
			return fmt.Errorf("boom")
		}
		cfg, err := Read(path)
		if err != nil {
			return err
		}
		SetCurrent(cfg)
		return nil
	})

	if _, err := manager.Update(func(cfg *Config) error {
		cfg.CoolingSeconds = 7
		return nil
	}); err == nil {
		t.Fatal("Update() error = nil, want reload failure")
	}
	if reloads != 2 {
		t.Fatalf("reloads = %d, want 2 because rollback should restore runtime", reloads)
	}

	loaded, err := Read(path)
	if err != nil {
		t.Fatalf("Read(path) error = %v", err)
	}
	if loaded.CoolingSeconds != initial.CoolingSeconds {
		t.Fatalf("loaded.CoolingSeconds = %d, want %d after rollback", loaded.CoolingSeconds, initial.CoolingSeconds)
	}
}

func TestManagerUpdateTreatsStripToolsChangeAsProviderChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	initial := mustNormalizedConfig(t, &Config{
		ActiveProvider: "p1",
		Providers: []ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}, StripTools: false},
		},
	})
	if err := writeFileAtomic(path, initial); err != nil {
		t.Fatalf("writeFileAtomic() error = %v", err)
	}

	reloads := 0
	manager := NewManager(path, func(path string) error {
		reloads++
		cfg, err := Read(path)
		if err != nil {
			return err
		}
		SetCurrent(cfg)
		return nil
	})

	result, err := manager.Update(func(cfg *Config) error {
		cfg.Providers[0].StripTools = true
		return nil
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if reloads != 1 {
		t.Fatalf("reloads = %d, want 1", reloads)
	}
	if result == nil || len(result.ChangedFields) != 1 || result.ChangedFields[0] != "providers" {
		t.Fatalf("result.ChangedFields = %#v, want [providers]", result)
	}

	loaded, err := Read(path)
	if err != nil {
		t.Fatalf("Read(path) error = %v", err)
	}
	if !loaded.Providers[0].StripTools {
		t.Fatal("loaded.Providers[0].StripTools = false, want true")
	}
}

// mustNormalizedConfig 把测试配置补齐为和正式运行一致的归一化状态。
func mustNormalizedConfig(t *testing.T, cfg *Config) *Config {
	t.Helper()
	next := cfg.Clone()
	if err := next.validate(); err != nil {
		t.Fatalf("validate() error = %v", err)
	}
	next.applyDefaults()
	if err := next.validateAfterDefaults(); err != nil {
		t.Fatalf("validateAfterDefaults() error = %v", err)
	}
	return next
}

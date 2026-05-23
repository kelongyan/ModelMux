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
	return &file, nil
}

// ProviderRecords 返回按 provider 分组的状态；旧版状态会映射到指定 provider。
func (f *File) ProviderRecords(defaultProviderID string) []ProviderRecord {
	if len(f.Providers) > 0 {
		return append([]ProviderRecord(nil), f.Providers...)
	}
	if len(f.Keys) == 0 {
		return nil
	}
	return []ProviderRecord{{
		ID:   defaultProviderID,
		Keys: append([]KeyRecord(nil), f.Keys...),
	}}
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
		if removeErr := os.Remove(s.path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("remove old state: %w", removeErr)
		}
		if renameErr := os.Rename(tmpPath, s.path); renameErr != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("replace state: %w", renameErr)
		}
	}
	return nil
}

// KeyID 计算 key 的稳定哈希标识，状态文件不保存完整 key。
func KeyID(key string) string {
	sum := sha256.Sum256([]byte(key))
	return "sha256:" + hex.EncodeToString(sum[:])
}

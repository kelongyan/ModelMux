package main

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kelongyan/ModelMux/config"
	"github.com/kelongyan/ModelMux/state"
)

func TestLoggerWriterStdout(t *testing.T) {
	writer, err := loggerWriter(&config.Config{LogOutput: "stdout"})
	if err != nil {
		t.Fatalf("loggerWriter() error = %v", err)
	}
	if writer != os.Stdout {
		t.Fatalf("loggerWriter() = %T, want os.Stdout", writer)
	}
}

func TestNewRollingLogWriterCreatesDirectory(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "nested", "proxy.log")
	writer, err := newRollingLogWriter(&config.Config{
		LogFile:       logFile,
		LogMaxSizeMB:  1,
		LogMaxBackups: 1,
		LogMaxAgeDays: 1,
	})
	if err != nil {
		t.Fatalf("newRollingLogWriter() error = %v", err)
	}
	if closer, ok := writer.(interface{ Close() error }); ok {
		defer func() {
			if err := closer.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		}()
	}

	if _, err := writer.Write([]byte("hello\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if _, err := os.Stat(logFile); err != nil {
		t.Fatalf("log file not created: %v", err)
	}
}

func TestLoggerWriterRequiresLogFileForFileOutput(t *testing.T) {
	_, err := loggerWriter(&config.Config{LogOutput: "file"})
	if err == nil {
		t.Fatal("loggerWriter() error = nil, want missing log_file error")
	}
}

func TestLoggerWriterRejectsUnsupportedOutput(t *testing.T) {
	_, err := loggerWriter(&config.Config{LogOutput: "unknown"})
	if err == nil {
		t.Fatal("loggerWriter() error = nil, want unsupported output error")
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("loggerWriter() error = %v, want unsupported output error", err)
	}
}

func TestStateSaverTriggerAndClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := state.NewStore(path)
	var snapshots atomic.Int32
	saver := newStateSaver(store, func() []state.ProviderRecord {
		snapshots.Add(1)
		return []state.ProviderRecord{{
			ID: "default",
			Keys: []state.KeyRecord{{
				KeyID: state.KeyID("k1"),
				State: "active",
			}},
		}}
	}, 10*time.Millisecond, func(err error) {
		t.Fatalf("state saver error = %v", err)
	})

	saver.Trigger(false)
	time.Sleep(50 * time.Millisecond)
	if snapshots.Load() == 0 {
		t.Fatal("snapshot was not saved after debounced trigger")
	}

	if err := saver.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state file not created: %v", err)
	}
}

// TestStateSaverHighFrequencyTriggerNotStarved 验证高频 Trigger(false) 不会因 timer 被
// 反复重置而永远不落盘——这是旧防抖实现下计数器无法持久化的根因。
func TestStateSaverHighFrequencyTriggerNotStarved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := state.NewStore(path)
	var snapshots atomic.Int32
	saver := newStateSaver(store, func() []state.ProviderRecord {
		snapshots.Add(1)
		return nil
	}, 30*time.Millisecond, func(err error) {
		t.Fatalf("state saver error = %v", err)
	})
	t.Cleanup(func() { _ = saver.Close() })

	// 持续 100ms 高频触发；旧实现下 timer 永远被 Reset，snapshots 应为 0。
	done := make(chan struct{})
	go func() {
		defer close(done)
		deadline := time.Now().Add(100 * time.Millisecond)
		for time.Now().Before(deadline) {
			saver.Trigger(false)
		}
	}()
	<-done

	// 合并窗口最长 30ms，给一点调度宽容。
	deadline := time.Now().Add(200 * time.Millisecond)
	for snapshots.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if snapshots.Load() == 0 {
		t.Fatal("snapshot never saved under sustained high-frequency triggers")
	}
}

// TestStateSaverWindowRestartsAfterFire 验证一次窗口落盘后，下一次 Trigger 能重启新窗口。
// 旧实现里 callback 不会把 s.timer 置 nil，新的非 immediate Trigger 会走 Reset 分支。
func TestStateSaverWindowRestartsAfterFire(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := state.NewStore(path)
	var snapshots atomic.Int32
	saver := newStateSaver(store, func() []state.ProviderRecord {
		snapshots.Add(1)
		return nil
	}, 20*time.Millisecond, func(err error) {
		t.Fatalf("state saver error = %v", err)
	})
	t.Cleanup(func() { _ = saver.Close() })

	saver.Trigger(false)
	time.Sleep(60 * time.Millisecond)
	first := snapshots.Load()
	if first == 0 {
		t.Fatal("first window did not fire")
	}

	saver.Trigger(false)
	time.Sleep(60 * time.Millisecond)
	if snapshots.Load() <= first {
		t.Fatalf("second window did not fire; snapshots=%d, want > %d", snapshots.Load(), first)
	}
}

func TestNewStatsStoreFromConfigDisabled(t *testing.T) {
	disabled := false
	store, err := newStatsStoreFromConfig(&config.Config{StatsEnabled: &disabled})
	if err != nil {
		t.Fatalf("newStatsStoreFromConfig() error = %v", err)
	}
	if store != nil {
		t.Fatal("newStatsStoreFromConfig() returned store, want nil when disabled")
	}
}

func TestNewStatsStoreFromConfigUsesConfig(t *testing.T) {
	dir := t.TempDir()
	store, err := newStatsStoreFromConfig(&config.Config{
		StatsDir:              dir,
		StatsRetentionDays:    30,
		StatsMaxRecentRecords: 2,
	})
	if err != nil {
		t.Fatalf("newStatsStoreFromConfig() error = %v", err)
	}
	if store == nil {
		t.Fatal("newStatsStoreFromConfig() = nil, want store")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("stats dir not ready: %v", err)
	}
}

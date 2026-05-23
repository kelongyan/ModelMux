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

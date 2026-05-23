package config

import (
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestWatcherShouldReloadOnlyTargetConfigWrites(t *testing.T) {
	target := filepath.Clean(filepath.Join(t.TempDir(), "config.json"))
	w := &Watcher{path: target}

	if !w.shouldReload(fsnotify.Event{Name: target, Op: fsnotify.Write}) {
		t.Fatal("shouldReload() = false, want true for target write")
	}
	if !w.shouldReload(fsnotify.Event{Name: target, Op: fsnotify.Create}) {
		t.Fatal("shouldReload() = false, want true for target create")
	}
	if w.shouldReload(fsnotify.Event{Name: target, Op: fsnotify.Chmod}) {
		t.Fatal("shouldReload() = true, want false for chmod")
	}
	if w.shouldReload(fsnotify.Event{Name: filepath.Join(filepath.Dir(target), "other.json"), Op: fsnotify.Write}) {
		t.Fatal("shouldReload() = true, want false for other file")
	}
}

func TestSamePathAllowsCleanedEquivalentPath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "config.json")
	equivalent := filepath.Join(dir, ".", "config.json")

	if !samePath(target, equivalent) {
		t.Fatalf("samePath(%q, %q) = false, want true", target, equivalent)
	}
}

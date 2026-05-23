package config

import (
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/claude-key-proxy/logx"
	"github.com/fsnotify/fsnotify"
)

const defaultWatchDebounce = 300 * time.Millisecond

type Watcher struct {
	watcher  *fsnotify.Watcher
	done     chan struct{}
	once     sync.Once
	path     string
	reloadFn func(string) error
	debounce time.Duration
}

// Watch 监听配置文件所在目录，并在目标文件变化后防抖触发 reloadFn。
func Watch(path string, reloadFn func(string) error) (*Watcher, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(absPath)
	if err := fsWatcher.Add(dir); err != nil {
		_ = fsWatcher.Close()
		return nil, err
	}

	w := &Watcher{
		watcher:  fsWatcher,
		done:     make(chan struct{}),
		path:     filepath.Clean(absPath),
		reloadFn: reloadFn,
		debounce: defaultWatchDebounce,
	}
	go w.run()
	return w, nil
}

// Close 停止文件监听并释放 fsnotify 资源。
func (w *Watcher) Close() error {
	var err error
	w.once.Do(func() {
		close(w.done)
		err = w.watcher.Close()
	})
	return err
}

// run 消费 fsnotify 事件并对同一次保存动作做防抖合并。
func (w *Watcher) run() {
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}

	for {
		select {
		case <-w.done:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			if w.shouldReload(event) {
				resetTimer(timer, w.debounce)
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("config watch error", logx.Fields(logx.CategoryConfig, logx.EventConfigWatchError,
				"err", err,
			)...)
		case <-timer.C:
			if err := w.reloadFn(w.path); err != nil {
				slog.Error("config auto reload failed", logx.Fields(logx.CategoryConfig, logx.EventConfigReloadFailed,
					"path", w.path,
					"err", err,
				)...)
			}
		}
	}
}

// shouldReload 判断文件事件是否命中目标配置文件和需要 reload 的操作类型。
func (w *Watcher) shouldReload(event fsnotify.Event) bool {
	if !samePath(w.path, event.Name) {
		return false
	}
	return event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0
}

// resetTimer 安全重置防抖定时器，避免重复保存触发多次 reload。
func resetTimer(timer *time.Timer, debounce time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(debounce)
}

// samePath 按平台路径规则比较文件路径，兼容 Windows 大小写差异。
func samePath(a, b string) bool {
	absB, err := filepath.Abs(b)
	if err == nil {
		b = absB
	}
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	return a == b || strings.EqualFold(a, b)
}

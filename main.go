package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/kelongyan/ModelMux/admin"
	"github.com/kelongyan/ModelMux/config"
	"github.com/kelongyan/ModelMux/logx"
	"github.com/kelongyan/ModelMux/pool"
	"github.com/kelongyan/ModelMux/proxy"
	"github.com/kelongyan/ModelMux/state"
	"github.com/kelongyan/ModelMux/stats"
	"gopkg.in/natefinch/lumberjack.v2"
)

// main 加载配置并启动代理服务与管理服务。
func main() {
	configPath := flag.String("config", "config.json", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", logx.Fields(logx.CategoryConfig, logx.EventConfigLoadFailed,
			"err", err,
		)...)
		os.Exit(1)
	}

	if err := setupLogger(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "failed to setup logger: %v\n", err)
		os.Exit(1)
	}

	providerPools, err := pool.NewProviderPools(providerSpecsFromConfig(cfg.ProviderConfigs()), cfg.ActiveProvider)
	if err != nil {
		slog.Error("failed to create provider pools", logx.Fields(logx.CategoryLifecycle, logx.EventKeyPoolInitialized,
			"err", err,
		)...)
		os.Exit(1)
	}
	slog.Info("provider pools initialized", logx.Fields(logx.CategoryLifecycle, logx.EventKeyPoolInitialized,
		"providers", providerPools.ProviderCount(),
		"active_provider", providerPools.ActiveID(),
		"active_keys", cfg.TotalKeys(),
		"total_keys", cfg.TotalProviderKeys(),
	)...)
	eventBuffer := admin.NewEventBuffer(300)
	eventBuffer.Add("info", logx.CategoryLifecycle, logx.EventKeyPoolInitialized, "provider pools initialized", map[string]any{
		"providers":       providerPools.ProviderCount(),
		"active_provider": providerPools.ActiveID(),
	})

	statsStore, err := newStatsStoreFromConfig(cfg)
	if err != nil {
		slog.Warn("stats store init failed", logx.Fields(logx.CategoryStats, logx.EventStatsInitFailed,
			"path", cfg.StatsDir,
			"err", err,
		)...)
		eventBuffer.Add("warn", logx.CategoryStats, logx.EventStatsInitFailed, "stats store init failed", map[string]any{
			"path":  cfg.StatsDir,
			"error": err.Error(),
		})
	} else if statsStore != nil {
		slog.Info("stats store initialized", logx.Fields(logx.CategoryStats, logx.EventStatsInitialized,
			"path", cfg.StatsDir,
			"retention_days", cfg.StatsRetentionDays,
			"max_recent_records", cfg.StatsMaxRecentRecords,
		)...)
		eventBuffer.Add("info", logx.CategoryStats, logx.EventStatsInitialized, "stats store initialized", map[string]any{
			"path": cfg.StatsDir,
		})
	}

	var saver *stateSaver
	if cfg.StatePersistenceEnabled() {
		store := state.NewStore(cfg.StateFile)
		stateFile, err := store.Load()
		if err != nil {
			slog.Warn("state load failed", logx.Fields(logx.CategoryState, logx.EventStateLoadFailed,
				"path", cfg.StateFile,
				"err", err,
			)...)
			eventBuffer.Add("warn", logx.CategoryState, logx.EventStateLoadFailed, "state load failed", map[string]any{
				"path":  cfg.StateFile,
				"error": err.Error(),
			})
		} else {
			records := stateFile.VersionedProviderRecords(cfg.ActiveProvider)
			providerPools.Restore(records, time.Duration(cfg.InvalidTTLHours)*time.Hour)
			slog.Info("state restored", logx.Fields(logx.CategoryState, logx.EventStateRestored,
				"path", cfg.StateFile,
				"providers", len(records),
				"invalid_ttl_hours", cfg.InvalidTTLHours,
			)...)
			eventBuffer.Add("info", logx.CategoryState, logx.EventStateRestored, "state restored", map[string]any{
				"path": cfg.StateFile,
			})
		}
		saver = newStateSaver(store, providerPools.Snapshot, 2*time.Second, func(err error) {
			slog.Warn("state save failed", logx.Fields(logx.CategoryState, logx.EventStateSaveFailed,
				"path", cfg.StateFile,
				"err", err,
			)...)
		})
	}

	proxyHandler, err := proxy.NewHandler(providerPools, cfg)
	if err != nil {
		slog.Error("failed to create proxy handler", logx.Fields(logx.CategoryLifecycle, logx.EventHandlerCreateFailed,
			"err", err,
		)...)
		os.Exit(1)
	}
	if saver != nil {
		proxyHandler.SetStateChangeHook(saver.Trigger)
	}
	if statsStore != nil {
		proxyHandler.SetStatsRecorder(statsStore)
	}
	proxyHandler.SetEventRecorder(eventBuffer)

	// 代理服务允许长时间流式输出，因此只设置读头和空闲超时，不设置固定写超时。
	proxyMux := http.NewServeMux()
	proxyMux.Handle("/", proxyHandler)

	proxySrv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           proxyMux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// 管理服务是本地控制面，设置更保守的读头和空闲超时即可。
	adminMux := http.NewServeMux()
	// reloadMu 串行化所有 reload 路径：admin API (Manager.Update 内调用) 与 fsnotify watcher
	// 都会触发 reloadConfig，两条路径只各自持有自己的锁。整体 reload 需要原子地切换
	// providerPools、proxyHandler.runtime 和 config.current，必须串行执行避免运行时不一致。
	var reloadMu sync.Mutex
	reloadConfig := func(path string) error {
		reloadMu.Lock()
		defer reloadMu.Unlock()
		newCfg, err := config.Read(path)
		if err != nil {
			eventBuffer.Add("error", logx.CategoryConfig, logx.EventConfigReloadFailed, "config reload failed", map[string]any{
				"path":  path,
				"error": err.Error(),
			})
			return err
		}
		if err := proxy.ValidateConfig(newCfg); err != nil {
			eventBuffer.Add("error", logx.CategoryConfig, logx.EventConfigReloadFailed, "config reload failed", map[string]any{
				"path":  path,
				"error": err.Error(),
			})
			return err
		}
		if err := providerPools.Update(providerSpecsFromConfig(newCfg.ProviderConfigs()), newCfg.ActiveProvider); err != nil {
			eventBuffer.Add("error", logx.CategoryConfig, logx.EventConfigReloadFailed, "config reload failed", map[string]any{
				"path":  path,
				"error": err.Error(),
			})
			return err
		}
		if err := proxyHandler.UpdateConfig(newCfg); err != nil {
			eventBuffer.Add("error", logx.CategoryConfig, logx.EventConfigReloadFailed, "config reload failed", map[string]any{
				"path":  path,
				"error": err.Error(),
			})
			return err
		}
		config.SetCurrent(newCfg)
		if saver != nil {
			saver.Trigger(true)
		}
		slog.Info("config reloaded", logx.Fields(logx.CategoryConfig, logx.EventConfigReloaded,
			"providers", len(newCfg.Providers),
			"active_provider", newCfg.ActiveProvider,
			"active_keys", newCfg.TotalKeys(),
			"total_keys", newCfg.TotalProviderKeys(),
			"target", newCfg.TargetURL,
			"cooling_seconds", newCfg.CoolingSeconds,
			"max_retries", newCfg.MaxRetries,
			"request_timeout_seconds", newCfg.RequestTimeoutSeconds,
			"max_body_bytes", newCfg.MaxBodyBytes,
		)...)
		eventBuffer.Add("info", logx.CategoryConfig, logx.EventConfigReloaded, "config reloaded", map[string]any{
			"active_provider": newCfg.ActiveProvider,
			"providers":       len(newCfg.Providers),
		})
		return nil
	}
	configManager := config.NewManager(*configPath, reloadConfig)
	var adminStateChanged func(bool)
	if saver != nil {
		adminStateChanged = saver.Trigger
	}
	adminHandler := admin.NewHandler(providerPools, configManager, reloadConfig, eventBuffer, adminStateChanged)
	adminHandler.SetProviderHealthReader(proxyHandler)
	if statsStore != nil {
		adminHandler.SetStatsStore(statsStore)
	}
	adminHandler.Register(adminMux)

	var configWatcher *config.Watcher
	if watcher, err := config.Watch(*configPath, reloadConfig); err != nil {
		slog.Warn("config watch failed", logx.Fields(logx.CategoryConfig, logx.EventConfigWatchFailed,
			"path", *configPath,
			"err", err,
		)...)
		eventBuffer.Add("warn", logx.CategoryConfig, logx.EventConfigWatchFailed, "config watch failed", map[string]any{
			"path":  *configPath,
			"error": err.Error(),
		})
	} else {
		configWatcher = watcher
		configManager.SetWatcher(watcher)
		slog.Info("config watch started", logx.Fields(logx.CategoryConfig, logx.EventConfigWatchStarted,
			"path", *configPath,
		)...)
		eventBuffer.Add("info", logx.CategoryConfig, logx.EventConfigWatchStarted, "config watch started", map[string]any{
			"path": *configPath,
		})
	}

	if cfg.AdminAPIKey == "" {
		slog.Warn("admin API has no authentication configured; all management endpoints are open", logx.Fields(logx.CategoryAdmin, "admin.no_auth",
			"hint", "set admin_api_key in config to enable authentication",
		)...)
	}

	adminSrv := &http.Server{
		Addr:              cfg.AdminListen,
		Handler:           adminMux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("proxy listening", logx.Fields(logx.CategoryLifecycle, logx.EventProxyListening,
			"addr", cfg.Listen,
			"active_provider", cfg.ActiveProvider,
			"target", cfg.TargetURL,
		)...)
		eventBuffer.Add("info", logx.CategoryLifecycle, logx.EventProxyListening, "proxy listening", map[string]any{
			"addr":            cfg.Listen,
			"active_provider": cfg.ActiveProvider,
		})
		if err := proxySrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("proxy server error", logx.Fields(logx.CategoryLifecycle, logx.EventServerError,
				"server", "proxy",
				"err", err,
			)...)
			os.Exit(1)
		}
	}()

	go func() {
		slog.Info("admin listening", logx.Fields(logx.CategoryLifecycle, logx.EventAdminListening,
			"addr", cfg.AdminListen,
		)...)
		eventBuffer.Add("info", logx.CategoryLifecycle, logx.EventAdminListening, "admin listening", map[string]any{
			"addr": cfg.AdminListen,
		})
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("admin server error", logx.Fields(logx.CategoryLifecycle, logx.EventServerError,
				"server", "admin",
				"err", err,
			)...)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	slog.Info("shutting down", logx.Fields(logx.CategoryLifecycle, logx.EventShutdownStart)...)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = proxySrv.Shutdown(ctx)
	_ = adminSrv.Shutdown(ctx)
	if configWatcher != nil {
		_ = configWatcher.Close()
	}
	if saver != nil {
		if err := saver.Close(); err != nil {
			slog.Warn("state save failed", logx.Fields(logx.CategoryState, logx.EventStateSaveFailed,
				"path", cfg.StateFile,
				"err", err,
			)...)
		}
	}
	if statsStore != nil {
		if err := statsStore.Close(); err != nil {
			slog.Warn("stats store close failed", logx.Fields(logx.CategoryStats, logx.EventStatsCloseFailed,
				"path", cfg.StatsDir,
				"err", err,
			)...)
		}
	}
	slog.Info("stopped", logx.Fields(logx.CategoryLifecycle, logx.EventShutdownComplete)...)
}

// providerSpecsFromConfig 提取 provider 的 ID 和 key 列表，用于创建或更新独立 key 池。
func providerSpecsFromConfig(providers []config.ProviderConfig) []pool.ProviderSpec {
	specs := make([]pool.ProviderSpec, 0, len(providers))
	for _, provider := range providers {
		specs = append(specs, pool.ProviderSpec{
			ID:   provider.ID,
			Keys: provider.EnabledKeys(),
		})
	}
	return specs
}

func newStatsStoreFromConfig(cfg *config.Config) (*stats.Store, error) {
	if cfg == nil || !cfg.StatsCollectionEnabled() {
		return nil, nil
	}
	dir := cfg.StatsDir
	if dir == "" {
		dir = config.DefaultStatsDir
	}
	retentionDays := cfg.StatsRetentionDays
	if retentionDays <= 0 {
		retentionDays = config.DefaultStatsRetentionDays
	}
	maxRecentRecords := cfg.StatsMaxRecentRecords
	if maxRecentRecords <= 0 {
		maxRecentRecords = config.DefaultStatsMaxRecentRecords
	}
	return stats.NewStore(stats.Options{
		Dir:              dir,
		RetentionDays:    retentionDays,
		MaxRecentRecords: maxRecentRecords,
	})
}

// setupLogger 根据配置初始化 slog 文本或 JSON 日志输出，并按需启用日志轮转。
func setupLogger(cfg *config.Config) error {
	var l slog.Level
	switch cfg.LogLevel {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}

	writer, err := loggerWriter(cfg)
	if err != nil {
		return err
	}

	opts := &slog.HandlerOptions{
		Level:     l,
		AddSource: l == slog.LevelDebug,
	}
	var handler slog.Handler
	if cfg.LogFormat == "json" {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}
	slog.SetDefault(slog.New(handler))
	return nil
}

// loggerWriter 根据 log_output 构造日志写入目标；文件目标使用 lumberjack 自动轮转。
func loggerWriter(cfg *config.Config) (io.Writer, error) {
	switch cfg.LogOutput {
	case "stdout", "":
		return os.Stdout, nil
	case "file":
		return newRollingLogWriter(cfg)
	case "both":
		fileWriter, err := newRollingLogWriter(cfg)
		if err != nil {
			return nil, err
		}
		return io.MultiWriter(os.Stdout, fileWriter), nil
	default:
		return nil, fmt.Errorf("unsupported log_output %q", cfg.LogOutput)
	}
}

// newRollingLogWriter 创建按大小、数量和天数轮转的日志文件写入器。
func newRollingLogWriter(cfg *config.Config) (io.Writer, error) {
	if cfg.LogFile == "" {
		return nil, fmt.Errorf("log_file is required for file logging")
	}
	if dir := filepath.Dir(cfg.LogFile); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create log directory: %w", err)
		}
	}
	return &lumberjack.Logger{
		Filename:   cfg.LogFile,
		MaxSize:    cfg.LogMaxSizeMB,
		MaxBackups: cfg.LogMaxBackups,
		MaxAge:     cfg.LogMaxAgeDays,
		Compress:   cfg.LogCompress,
	}, nil
}

type stateSaver struct {
	store    *state.Store
	snapshot func() []state.ProviderRecord
	delay    time.Duration
	onError  func(error)
	mu       sync.Mutex
	saveMu   sync.Mutex
	timer    *time.Timer
	closed   bool
}

// newStateSaver 创建防抖状态保存器，避免每次成功请求都同步写磁盘。
func newStateSaver(store *state.Store, snapshot func() []state.ProviderRecord, delay time.Duration, onError func(error)) *stateSaver {
	return &stateSaver{
		store:    store,
		snapshot: snapshot,
		delay:    delay,
		onError:  onError,
	}
}

// Trigger 根据 immediate 决定立即保存或合并窗口保存。
// 非 immediate 时落入当前合并窗口（已有 pending timer 就不再重置），
// 避免高 QPS 下 timer 被反复 reset 导致永不落盘。
func (s *stateSaver) Trigger(immediate bool) {
	if immediate {
		if err := s.SaveNow(); err != nil && s.onError != nil {
			s.onError(err)
		}
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if s.timer != nil {
		return
	}
	s.timer = time.AfterFunc(s.delay, s.flushAfterTimer)
}

// flushAfterTimer 在合并窗口到期时触发；先清空 timer 让下一窗口能启动，再落盘。
func (s *stateSaver) flushAfterTimer() {
	s.mu.Lock()
	s.timer = nil
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return
	}
	if err := s.SaveNow(); err != nil && s.onError != nil {
		s.onError(err)
	}
}

// SaveNow 立即保存当前 key 池快照。
func (s *stateSaver) SaveNow() error {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	return s.store.Save(s.snapshot())
}

// Close 停止防抖定时器并保存最后一次状态。
func (s *stateSaver) Close() error {
	s.mu.Lock()
	s.closed = true
	if s.timer != nil {
		s.timer.Stop()
	}
	s.mu.Unlock()
	return s.SaveNow()
}

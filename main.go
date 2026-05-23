package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/claude-key-proxy/admin"
	"github.com/claude-key-proxy/config"
	"github.com/claude-key-proxy/pool"
	"github.com/claude-key-proxy/proxy"
)

func main() {
	configPath := flag.String("config", "config.json", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	setupLogger(cfg.LogLevel, cfg.LogFormat)

	keyPool := pool.New(cfg.Keys)
	slog.Info("key pool initialized", "total", keyPool.TotalCount())

	proxyHandler, err := proxy.NewHandler(keyPool, cfg)
	if err != nil {
		slog.Error("failed to create proxy handler", "err", err)
		os.Exit(1)
	}

	// Proxy server
	proxyMux := http.NewServeMux()
	proxyMux.Handle("/", proxyHandler)

	proxySrv := &http.Server{
		Addr:    cfg.Listen,
		Handler: proxyMux,
	}

	// Admin server (separate port)
	adminMux := http.NewServeMux()
	adminHandler := admin.NewHandler(keyPool, *configPath, func(path string) error {
		newCfg, err := config.Reload(path)
		if err != nil {
			return err
		}
		keyPool.Update(newCfg.Keys)
		slog.Info("config reloaded", "keys", newCfg.TotalKeys())
		return nil
	})
	adminHandler.Register(adminMux)

	adminSrv := &http.Server{
		Addr:    cfg.AdminListen,
		Handler: adminMux,
	}

	go func() {
		slog.Info("proxy listening", "addr", cfg.Listen, "target", cfg.TargetURL)
		if err := proxySrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("proxy server error", "err", err)
			os.Exit(1)
		}
	}()

	go func() {
		slog.Info("admin listening", "addr", cfg.AdminListen)
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("admin server error", "err", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	slog.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = proxySrv.Shutdown(ctx)
	_ = adminSrv.Shutdown(ctx)
	slog.Info("stopped")
}

func setupLogger(level, format string) {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: l}
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}

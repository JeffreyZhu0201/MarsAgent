// gateway server 的入口。装载 config → 构造 deps → 起 HTTP server。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/marsagent/gateway/internal/api"
	"github.com/marsagent/gateway/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           api.NewRouter(api.Deps{}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// 起一个独立 goroutine 跑 server，主 goroutine 等信号。
	go func() {
		slog.Info("gateway listening", "addr", srv.Addr, "version", cfg.ServerVersion)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()

	// 优雅关闭：SIGINT / SIGTERM 后给 10s 清理时间。
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	slog.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "err", err)
		os.Exit(1)
	}
	slog.Info("gateway stopped")
}
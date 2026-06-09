// gateway server 的入口。装载 config → 构造 deps → 起 HTTP server。
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/marsagent/gateway/internal/api"
	"github.com/marsagent/gateway/internal/config"
	"github.com/marsagent/gateway/internal/grpcc"
	"github.com/marsagent/gateway/internal/sandbox"
	"github.com/marsagent/gateway/internal/store"
	"github.com/marsagent/gateway/internal/stream"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}

	rdb := redis.NewClient(mustParseRedis(cfg.RedisURL))
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		slog.Error("redis ping failed", "err", err)
		os.Exit(1)
	}

	db, err := sql.Open("postgres",
		os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("db open failed", "err", err)
		os.Exit(1)
	}
	if err := db.Ping(); err != nil {
		slog.Error("db ping failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	wc, err := grpcc.Dial(cfg.GRPCTarget)
	if err != nil {
		slog.Error("grpc dial failed", "target", cfg.GRPCTarget, "err", err)
		os.Exit(1)
	}
	defer wc.Close()

	sch, err := sandbox.NewScheduler()
	if err != nil {
		slog.Warn("sandbox scheduler init failed (docker may not be available)", "err", err)
		sch = nil
	}

	deps := api.Deps{
		Producer:    stream.NewRedisProducer(rdb),
		Subscriber:  stream.NewRedisSubscriber(rdb),
		GRPC:        wc,
		DB:          db,
		CourseStore: store.NewCourseStore(db),
		WikiStore:   store.NewWikiStore(db),
		Sandbox:     sch,
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           api.NewRouter(deps),
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

func mustParseRedis(url string) *redis.Options {
	opts, err := redis.ParseURL(url)
	if err != nil {
		slog.Error("redis url parse failed", "url", url, "err", err)
		os.Exit(1)
	}
	return opts
}

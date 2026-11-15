// api 入口：装配依赖、执行迁移与种子、启动 Outbox 投递器与 HTTP 服务。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/modules/follows"
	"github.com/yanji-community/backend/internal/modules/moderation"
	"github.com/yanji-community/backend/internal/modules/notifications"
	"github.com/yanji-community/backend/internal/platform/config"
	"github.com/yanji-community/backend/internal/platform/database"
	"github.com/yanji-community/backend/internal/platform/logger"
	"github.com/yanji-community/backend/internal/platform/mailer"
	"github.com/yanji-community/backend/internal/platform/migrate"
	"github.com/yanji-community/backend/internal/platform/redisdb"
	"github.com/yanji-community/backend/internal/platform/seed"
	"github.com/yanji-community/backend/internal/platform/storage"
	"github.com/yanji-community/backend/internal/server"
	"github.com/yanji-community/backend/internal/shared/outbox"
	"github.com/yanji-community/backend/internal/shared/rbac"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logger.New(cfg.Env)
	log.Info("starting api", "env", cfg.Env, "port", cfg.Port)

	// 数据库
	pool, err := database.New(ctx, cfg.DatabaseURL, cfg.DatabaseMaxConns)
	if err != nil {
		log.Error("database unavailable", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// 迁移 + 种子
	if cfg.MigrateOnStart {
		if err := migrate.Up(ctx, cfg.DatabaseURL); err != nil {
			log.Error("migration failed", "err", err)
			os.Exit(1)
		}
		log.Info("migrations applied")
	}
	if err := seed.Run(ctx, pool, cfg, log); err != nil {
		log.Error("seed failed", "err", err)
		os.Exit(1)
	}

	// Redis（开发环境允许不可用，限流/去重自动降级）
	redisCli, err := redisdb.New(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Warn("redis unavailable, rate limiting degraded", "err", err)
		redisCli = nil
	} else {
		defer redisCli.Close()
	}

	// 存储（local 驱动；S3 驱动预留接口，生产可扩展实现）
	var store storage.Storage
	store, err = storage.NewLocalStorage(cfg.StorageLocalDir, cfg.StoragePublicBase)
	if err != nil {
		log.Error("storage init failed", "err", err)
		os.Exit(1)
	}

	// 邮件
	var mailerSvc mailer.Mailer
	if cfg.MailDriver == "smtp" && cfg.SMTPHost != "" {
		mailerSvc = mailer.NewSMTPMailer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom, log)
	} else {
		mailerSvc = mailer.NewLogMailer(log)
	}

	// Outbox 投递器（API 进程内嵌；独立 worker 同构部署见 cmd/worker）
	dispatcher := outbox.NewDispatcher(pool, log)
	notifications.RegisterConsumer(dispatcher, pool, redisCli)
	notifications.RegisterEmailHandlers(dispatcher, mailerSvc, cfg)
	go dispatcher.Run(ctx)

	// 处罚过期惰性解除（每 10 分钟）
	go runSanctionExpiry(ctx, pool, log)

	// 跨模块钩子：首次关注用户 → 通知
	follows.OnUserFollowed = func(ctx context.Context, followerID, targetID int64) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return
		}
		defer tx.Rollback(ctx)
		_ = notifications.EmitTx(ctx, tx, notifications.Payload{
			RecipientID: targetID, ActorID: followerID, Type: notifications.TypeFollow,
			ResourceType: "user", ResourceID: followerID,
			Title:     "你有了一位新粉丝",
			DedupeKey: "follow:" + strconv.FormatInt(followerID, 10) + ":" + strconv.FormatInt(targetID, 10),
		})
		_ = tx.Commit(ctx)
	}

	// RBAC 与治理
	rbacSvc := rbac.New(pool)
	audit := moderation.NewAuditLogger(pool, log)
	gate := moderation.NewGate(pool)

	// HTTP 服务
	engine := server.Build(server.Deps{
		Cfg: cfg, Pool: pool, Redis: redisCli, Logger: log,
		RBAC: rbacSvc, Audit: audit, Gate: gate, Store: store,
	})
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		log.Info("http listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server error", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Info("bye")
}

// runSanctionExpiry 周期性将过期处罚标记为 expired。
func runSanctionExpiry(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	svc := moderation.NewSanctionService(pool, log)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := svc.ExpireDue(ctx); err == nil && n > 0 {
				log.Info("sanctions expired", "count", n)
			}
		}
	}
}

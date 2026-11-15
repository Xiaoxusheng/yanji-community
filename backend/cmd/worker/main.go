// worker 入口：独立进程消费 Outbox 事件与周期任务（与 API 进程同构消费者，
// outbox 采用 FOR UPDATE SKIP LOCKED 认领，多进程并发不重复投递）。
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yanji-community/backend/internal/modules/moderation"
	"github.com/yanji-community/backend/internal/modules/notifications"
	"github.com/yanji-community/backend/internal/platform/config"
	"github.com/yanji-community/backend/internal/platform/database"
	"github.com/yanji-community/backend/internal/platform/logger"
	"github.com/yanji-community/backend/internal/platform/mailer"
	"github.com/yanji-community/backend/internal/platform/migrate"
	"github.com/yanji-community/backend/internal/platform/seed"
	"github.com/yanji-community/backend/internal/shared/outbox"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logger.New(cfg.Env)
	log.Info("starting worker", "env", cfg.Env)

	pool, err := database.New(ctx, cfg.DatabaseURL, cfg.DatabaseMaxConns)
	if err != nil {
		log.Error("database unavailable", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if cfg.MigrateOnStart {
		if err := migrate.Up(ctx, cfg.DatabaseURL); err != nil {
			log.Error("migration failed", "err", err)
			os.Exit(1)
		}
	}
	if err := seed.Run(ctx, pool, cfg, log); err != nil {
		log.Error("seed failed", "err", err)
		os.Exit(1)
	}

	var mailerSvc mailer.Mailer
	if cfg.MailDriver == "smtp" && cfg.SMTPHost != "" {
		mailerSvc = mailer.NewSMTPMailer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom, log)
	} else {
		mailerSvc = mailer.NewLogMailer(log)
	}

	dispatcher := outbox.NewDispatcher(pool, log)
	notifications.RegisterConsumer(dispatcher, pool, nil) // worker 无 Redis，实时推送由 API 进程完成
	notifications.RegisterEmailHandlers(dispatcher, mailerSvc, cfg)
	go dispatcher.Run(ctx)

	sanctions := moderation.NewSanctionService(pool, log)
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	log.Info("worker running")
	for {
		select {
		case <-ctx.Done():
			log.Info("worker stopped")
			return
		case <-ticker.C:
			if n, err := sanctions.ExpireDue(ctx); err == nil && n > 0 {
				log.Info("sanctions expired", "count", n)
			}
		}
	}
}

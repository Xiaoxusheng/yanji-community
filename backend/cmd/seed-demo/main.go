// seed-demo 入口：幂等初始化基础种子与社区演示测试数据（不覆盖已有用户与内容）。
package main

import (
	"context"
	"os"

	"github.com/yanji-community/backend/internal/platform/config"
	"github.com/yanji-community/backend/internal/platform/database"
	"github.com/yanji-community/backend/internal/platform/logger"
	"github.com/yanji-community/backend/internal/platform/migrate"
	"github.com/yanji-community/backend/internal/platform/seed"
)

func main() {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logger.New(cfg.Env)

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
		log.Error("core seed failed", "err", err)
		os.Exit(1)
	}
	if err := seed.RunDemo(ctx, pool, log); err != nil {
		log.Error("demo seed failed", "err", err)
		os.Exit(1)
	}
}

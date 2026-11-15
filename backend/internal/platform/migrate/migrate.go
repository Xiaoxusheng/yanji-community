// Package migrate 使用 golang-migrate 执行内嵌 SQL 迁移。
package migrate

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/yanji-community/backend/migrations"
)

// Up 将数据库升级到最新版本；无新迁移时为幂等 no-op。
func Up(ctx context.Context, databaseURL string) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("读取内嵌迁移失败: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, "pgx5://"+trimScheme(databaseURL))
	if err != nil {
		return fmt.Errorf("初始化迁移失败: %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("执行迁移失败: %w", err)
	}
	return nil
}

// golang-migrate 的 pgx5 driver 接受 postgres:// 之外的 url 形式，这里统一前缀。
func trimScheme(databaseURL string) string {
	for _, p := range []string{"postgres://", "postgresql://", "pgx5://", "pgx://"} {
		if len(databaseURL) >= len(p) && databaseURL[:len(p)] == p {
			return databaseURL[len(p):]
		}
	}
	return databaseURL
}

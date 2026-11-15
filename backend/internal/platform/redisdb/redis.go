// Package redisdb 管理 Redis 客户端（限流、幂等键、去重）。
// Redis 不作为业务数据源，仅承载可丢失的短期状态。
package redisdb

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// New 创建并验证 Redis 客户端。
func New(ctx context.Context, addr, password string, db int) (*redis.Client, error) {
	cli := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := cli.Ping(pingCtx).Err(); err != nil {
		return nil, fmt.Errorf("Redis 连接失败: %w", err)
	}
	return cli, nil
}

package auth

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestAuthUIDAndChangePasswordLockout(t *testing.T) {
	svc := New(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Hour, "http://localhost:3000")
	ctx := context.Background()

	// 1. 交替用户名与邮箱尝试登录，通过 uid 维度计数防止尝试上限翻倍
	uidKey := "uid:42"
	for i := 0; i < loginMaxFailPerPair; i++ {
		if err := svc.checkLoginLockout(ctx, uidKey, "198.51.100.1"); err != nil {
			t.Fatalf("第 %d 次 uid 维度尝试前不应锁定: %v", i+1, err)
		}
		svc.recordLoginFailure(ctx, uidKey, "198.51.100.1")
	}
	if err := svc.checkLoginLockout(ctx, uidKey, "198.51.100.1"); err == nil {
		t.Fatal("达到 uid 维度失败阈值后必须触发锁定")
	}

	// 2. 修改密码（chpwd:uid）独立防暴破锁定与成功后清零
	chpwdKey := "chpwd:42"
	for i := 0; i < loginMaxFailPerPair; i++ {
		svc.recordLoginFailure(ctx, chpwdKey, "198.51.100.2")
	}
	if err := svc.checkLoginLockout(ctx, chpwdKey, "198.51.100.2"); err == nil {
		t.Fatal("连续输错旧密码达到阈值必须触发锁定")
	}
	svc.clearLoginFailure(ctx, chpwdKey, "198.51.100.2")
	if err := svc.checkLoginLockout(ctx, chpwdKey, "198.51.100.2"); err != nil {
		t.Fatalf("清除修改密码失败计数后不应继续锁定: %v", err)
	}
}

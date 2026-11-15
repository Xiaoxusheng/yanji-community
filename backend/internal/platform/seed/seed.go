// Package seed 初始化基础数据：角色/权限、默认版块、系统配置、管理员账号、欢迎公告。
// 幂等：全部使用 ON CONFLICT DO NOTHING。
package seed

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/yanji-community/backend/internal/platform/config"
)

var permissions = []struct{ Code, Name, Resource, Action string }{
	{"post.create", "发帖", "post", "create"},
	{"post.comment", "评论", "post", "comment"},
	{"post.interact", "点赞收藏关注", "post", "interact"},
	{"post.moderate", "审核帖子", "post", "moderate"},
	{"comment.moderate", "审核评论", "comment", "moderate"},
	{"post.pin", "置顶帖子", "post", "pin"},
	{"post.feature", "加精帖子", "post", "feature"},
	{"report.create", "提交举报", "report", "create"},
	{"report.handle", "处理举报", "report", "handle"},
	{"dashboard.read", "查看仪表盘", "dashboard", "read"},
	{"user.read", "查看用户", "user", "read"},
	{"user.sanction", "处罚用户", "user", "sanction"},
	{"category.manage", "管理版块", "category", "manage"},
	{"tag.manage", "管理标签", "tag", "manage"},
	{"announcement.manage", "管理公告", "announcement", "manage"},
	{"moderation.manage", "治理配置", "moderation", "manage"},
	{"role.read", "查看角色", "role", "read"},
	{"role.assign", "授予角色", "role", "assign"},
	{"settings.manage", "系统设置", "settings", "manage"},
	{"audit.read", "查看审计日志", "audit", "read"},
}

// rolePerms 角色权限映射。
var rolePerms = map[string][]string{
	"member": {"post.create", "post.comment", "post.interact", "report.create"},
	"moderator": {"post.create", "post.comment", "post.interact", "report.create",
		"post.moderate", "comment.moderate", "post.pin", "post.feature", "report.handle"},
	"admin": {"post.create", "post.comment", "post.interact", "report.create",
		"post.moderate", "comment.moderate", "post.pin", "post.feature", "report.handle",
		"dashboard.read", "user.read", "user.sanction", "category.manage", "tag.manage",
		"announcement.manage", "moderation.manage", "role.read", "settings.manage", "audit.read"},
	"superadmin": {}, // 全部权限，下方循环填入
}

// Run 执行种子数据写入。
func Run(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, logger *slog.Logger) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 角色
	for _, r := range []struct{ code, name, desc string }{
		{"member", "注册用户", "普通注册用户"},
		{"moderator", "版主", "版块范围管理"},
		{"admin", "管理员", "站点管理"},
		{"superadmin", "超级管理员", "全部权限"},
	} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO roles (code, name, description, is_system)
			VALUES ($1, $2, $3, TRUE) ON CONFLICT (code) DO NOTHING`, r.code, r.name, r.desc); err != nil {
			return err
		}
	}
	// 权限点
	for _, p := range permissions {
		if _, err := tx.Exec(ctx, `
			INSERT INTO permissions (code, name, resource, action)
			VALUES ($1, $2, $3, $4) ON CONFLICT (code) DO NOTHING`,
			p.Code, p.Name, p.Resource, p.Action); err != nil {
			return err
		}
	}
	// 角色权限
	allPerms := make([]string, 0, len(permissions))
	for _, p := range permissions {
		allPerms = append(allPerms, p.Code)
	}
	rolePerms["superadmin"] = allPerms
	for role, codes := range rolePerms {
		for _, code := range codes {
			if _, err := tx.Exec(ctx, `
				INSERT INTO role_permissions (role_id, permission_id)
				SELECT r.id, p.id FROM roles r, permissions p
				WHERE r.code = $1 AND p.code = $2
				ON CONFLICT DO NOTHING`, role, code); err != nil {
				return err
			}
		}
	}
	// 默认版块
	for _, c := range []struct {
		slug, name, desc string
		sort             int
	}{
		{"general", "综合讨论", "畅所欲言的综合讨论区", 1},
		{"tech", "技术分享", "编程、开发与工具交流", 2},
		{"life", "生活日常", "记录与分享生活点滴", 3},
		{"site", "站务公告", "站点公告与反馈建议", 100},
	} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO categories (name, slug, description, sort_order)
			VALUES ($1, $2, $3, $4) ON CONFLICT (slug) DO NOTHING`, c.name, c.slug, c.desc, c.sort); err != nil {
			return err
		}
	}
	// 系统配置
	for _, sc := range []struct {
		key, value, typ, desc string
		public                bool
	}{
		{"site_name", "研记社区", "string", "站点名称", true},
		{"site_description", "一个开放注册的综合社区论坛", "string", "站点描述", true},
		{"posts_max_tags", "5", "int", "每帖标签数量上限", true},
		{"title_min_len", "5", "int", "标题最小长度", false},
		{"require_email_to_post", "true", "bool", "发帖需验证邮箱", false},
	} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO system_configs (key, value, value_type, description, is_public)
			VALUES ($1, $2, $3, $4, $5) ON CONFLICT (key) DO NOTHING`,
			sc.key, sc.value, sc.typ, sc.desc, sc.public); err != nil {
			return err
		}
	}
	// 管理员账号
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.SeedAdminPassword), 12)
	if err != nil {
		return err
	}
	var adminID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO users (username, email_norm, password_hash, status, email_verified_at)
		VALUES ('admin', lower($1), $2, 'active', now_utc())
		ON CONFLICT (username) DO UPDATE SET updated_at = now_utc()
		RETURNING id`, cfg.SeedAdminEmail, string(hash)).Scan(&adminID)
	if err != nil {
		return fmt.Errorf("种子管理员创建失败: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO user_profiles (user_id, display_name) VALUES ($1, '管理员') ON CONFLICT (user_id) DO NOTHING`, adminID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id, scope_type)
		SELECT $1, id, 'global' FROM roles WHERE code IN ('member', 'admin', 'superadmin')
		ON CONFLICT DO NOTHING`, adminID); err != nil {
		return err
	}
	// 欢迎公告
	var annCount int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM announcements`).Scan(&annCount); err != nil {
		return err
	}
	if annCount == 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO announcements (title, body, status, author_id, published_at)
			VALUES ('欢迎来到研记社区',
			  '这里是开放注册的综合社区。发帖前请先完成邮箱验证；请遵守社区规则，尊重每一位成员。',
			  'published', $1, now_utc())`, adminID); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	logger.Info("seed data ready", "admin", cfg.SeedAdminEmail)
	return nil
}

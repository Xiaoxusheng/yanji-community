// Package rbac 实现 RBAC 权限查询与授权管理。
// 权限判断只在服务端执行；带内存缓存（5 分钟）并在授权变更时失效。
package rbac

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service RBAC 服务。
type Service struct {
	pool *pgxpool.Pool

	mu    sync.RWMutex
	cache map[int64]cachedPerms // key: userID
	ttl   time.Duration
}

type cachedPerms struct {
	codes       map[string]bool // 任意作用域授予的权限点
	globalCodes map[string]bool // 仅全局作用域授予的权限点
	moderated   map[int64]bool  // 版主授权版块
	expiresAt   time.Time
}

// New 构造 RBAC 服务。
func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, cache: map[int64]cachedPerms{}, ttl: 5 * time.Minute}
}

func (s *Service) load(ctx context.Context, userID int64) (*cachedPerms, error) {
	s.mu.RLock()
	if cp, ok := s.cache[userID]; ok && time.Now().Before(cp.expiresAt) {
		s.mu.RUnlock()
		return &cp, nil
	}
	s.mu.RUnlock()

	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT p.code, ur.scope_type, COALESCE(ur.scope_id, 0)
		FROM user_roles ur
		JOIN role_permissions rp ON rp.role_id = ur.role_id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE ur.user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cp := cachedPerms{codes: map[string]bool{}, globalCodes: map[string]bool{},
		moderated: map[int64]bool{}, expiresAt: time.Now().Add(s.ttl)}
	for rows.Next() {
		var code, scopeType string
		var scopeID int64
		if err := rows.Scan(&code, &scopeType, &scopeID); err != nil {
			return nil, err
		}
		cp.codes[code] = true
		if scopeType == "global" {
			cp.globalCodes[code] = true
		}
		if scopeType == "category" && scopeID > 0 {
			cp.moderated[scopeID] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cache[userID] = cp
	s.mu.Unlock()
	return &cp, nil
}

// HasPermission 判断用户是否拥有权限点（任意作用域）。
func (s *Service) HasPermission(ctx context.Context, userID int64, code string) (bool, error) {
	cp, err := s.load(ctx, userID)
	if err != nil {
		return false, err
	}
	return cp.codes[code], nil
}

// HasGlobalPermission 判断用户是否以全局作用域拥有权限点。
func (s *Service) HasGlobalPermission(ctx context.Context, userID int64, code string) (bool, error) {
	cp, err := s.load(ctx, userID)
	if err != nil {
		return false, err
	}
	return cp.globalCodes[code], nil
}

// CanModerateCategory 判断用户是否可管理指定版块：
// 全局作用域持有 post.moderate / category.manage 的用户不受版块范围限制；
// 否则检查版块范围授权集合。
func (s *Service) CanModerateCategory(ctx context.Context, userID int64, categoryID int64) (bool, error) {
	cp, err := s.load(ctx, userID)
	if err != nil {
		return false, err
	}
	if cp.globalCodes["post.moderate"] || cp.globalCodes["category.manage"] {
		return true, nil
	}
	return cp.moderated[categoryID], nil
}

// ModeratedCategoryIDs 返回用户以版块范围授权的版块 ID 集合。
func (s *Service) ModeratedCategoryIDs(ctx context.Context, userID int64) ([]int64, error) {
	cp, err := s.load(ctx, userID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(cp.moderated))
	for id := range cp.moderated {
		ids = append(ids, id)
	}
	return ids, nil
}

// Invalidate 清空指定用户的权限缓存。
func (s *Service) Invalidate(userID int64) {
	s.mu.Lock()
	delete(s.cache, userID)
	s.mu.Unlock()
}

// ---- 授权管理（管理后台使用）----

// Role 角色信息。
type Role struct {
	ID          int64    `json:"id"`
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	IsSystem    bool     `json:"is_system"`
	Permissions []string `json:"permissions"`
}

// ListRoles 返回全部角色及权限点。
func (s *Service) ListRoles(ctx context.Context) ([]Role, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.code, r.name, r.description, r.is_system,
		       COALESCE(array_agg(p.code) FILTER (WHERE p.code IS NOT NULL), '{}') AS perms
		FROM roles r
		LEFT JOIN role_permissions rp ON rp.role_id = r.id
		LEFT JOIN permissions p ON p.id = rp.permission_id
		GROUP BY r.id ORDER BY r.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Role
	for rows.Next() {
		var r Role
		if err := rows.Scan(&r.ID, &r.Code, &r.Name, &r.Description, &r.IsSystem, &r.Permissions); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListPermissions 返回全部权限点。
func (s *Service) ListPermissions(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, code, name, resource, action FROM permissions ORDER BY resource, action`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id int64
		var code, name, resource, action string
		if err := rows.Scan(&id, &code, &name, &resource, &action); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "code": code, "name": name, "resource": resource, "action": action})
	}
	return out, rows.Err()
}

// AssignRole 授予角色（scope 为版块时校验版块存在）。
func (s *Service) AssignRole(ctx context.Context, tx pgx.Tx, userID, roleID, scopeID int64, scopeType string) error {
	if scopeType == "" {
		scopeType = "global"
	}
	if scopeType == "global" {
		scopeID = 0
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id, scope_type, scope_id)
		VALUES ($1, $2, $3, NULLIF($4, 0))
		ON CONFLICT (user_id, role_id, scope_type, scope_id) DO NOTHING`,
		userID, roleID, scopeType, scopeID)
	if err != nil {
		return err
	}
	s.Invalidate(userID)
	return nil
}

// RemoveRole 移除角色授权。
func (s *Service) RemoveRole(ctx context.Context, tx pgx.Tx, userID, roleID int64, scopeID int64) error {
	tag, err := tx.Exec(ctx, `
		DELETE FROM user_roles
		WHERE user_id=$1 AND role_id=$2 AND COALESCE(scope_id,0) = $3`,
		userID, roleID, scopeID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("角色授权不存在")
	}
	s.Invalidate(userID)
	return nil
}

// UserRoles 用户角色明细（管理后台详情用）。
func (s *Service) UserRoles(ctx context.Context, userID int64) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT ur.id, r.code, r.name, ur.scope_type, COALESCE(ur.scope_id, 0),
		       COALESCE(c.name, '')
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		LEFT JOIN categories c ON c.id = ur.scope_id
		WHERE ur.user_id = $1 ORDER BY ur.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id int64
		var code, name, scopeType, catName string
		var scopeID int64
		if err := rows.Scan(&id, &code, &name, &scopeType, &scopeID, &catName); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "role_code": code, "role_name": name,
			"scope_type": scopeType, "scope_id": scopeID, "scope_name": catName,
		})
	}
	return out, rows.Err()
}

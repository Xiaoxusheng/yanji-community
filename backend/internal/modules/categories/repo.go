// Package categories 版块：树形列表、详情、后台 CRUD 与版主管理。
package categories

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/shared/apperr"
)

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$`)

// Category 版块。
type Category struct {
	ID            int64     `json:"id"`
	ParentID      *int64    `json:"parent_id"`
	ParentName    string    `json:"parent_name,omitempty"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	Description   string    `json:"description"`
	Icon          string    `json:"icon"`
	SortOrder     int       `json:"sort_order"`
	Status        string    `json:"status"`
	Visibility    string    `json:"visibility"`
	PostingPolicy string    `json:"posting_policy"`
	ReviewPolicy  string    `json:"review_policy"`
	PostCount     int64     `json:"post_count"`
	CreatedAt     time.Time `json:"created_at"`
}

// Repo 版块数据访问。
type Repo struct{ pool *pgxpool.Pool }

// NewRepo 构造。
func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

const categoryCols = `c.id, c.parent_id, COALESCE(pc.name, ''), c.name, c.slug, c.description, c.icon,
	c.sort_order, c.status, c.visibility, c.posting_policy, c.review_policy, c.post_count, c.created_at`

func scanCategory(row pgx.Row) (*Category, error) {
	var cat Category
	err := row.Scan(&cat.ID, &cat.ParentID, &cat.ParentName, &cat.Name, &cat.Slug, &cat.Description,
		&cat.Icon, &cat.SortOrder, &cat.Status, &cat.Visibility, &cat.PostingPolicy, &cat.ReviewPolicy,
		&cat.PostCount, &cat.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &cat, nil
}

// ListPublic 公开版块列表（排除 hidden/private/归档；private 仅返回给授权者由 service 过滤）。
func (r *Repo) ListPublic(ctx context.Context) ([]Category, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+categoryCols+`
		FROM categories c LEFT JOIN categories pc ON pc.id = c.parent_id
		WHERE c.deleted_at IS NULL AND c.status = 'active' AND c.visibility = 'public'
		ORDER BY c.sort_order, c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Category
	for rows.Next() {
		cat, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *cat)
	}
	return out, rows.Err()
}

// ListAll 后台全量列表。
func (r *Repo) ListAll(ctx context.Context) ([]Category, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+categoryCols+`
		FROM categories c LEFT JOIN categories pc ON pc.id = c.parent_id
		WHERE c.deleted_at IS NULL
		ORDER BY c.sort_order, c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Category
	for rows.Next() {
		cat, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *cat)
	}
	return out, rows.Err()
}

// GetBySlug 按 slug 查询。
func (r *Repo) GetBySlug(ctx context.Context, slug string) (*Category, error) {
	cat, err := scanCategory(r.pool.QueryRow(ctx, `
		SELECT `+categoryCols+`
		FROM categories c LEFT JOIN categories pc ON pc.id = c.parent_id
		WHERE c.slug = $1 AND c.deleted_at IS NULL`, slug))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("版块不存在")
	}
	return cat, err
}

// GetByID 内部 ID 查询。
func (r *Repo) GetByID(ctx context.Context, id int64) (*Category, error) {
	cat, err := scanCategory(r.pool.QueryRow(ctx, `
		SELECT `+categoryCols+`
		FROM categories c LEFT JOIN categories pc ON pc.id = c.parent_id
		WHERE c.id = $1 AND c.deleted_at IS NULL`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("版块不存在")
	}
	return cat, err
}

// Create 创建版块。
func (r *Repo) Create(ctx context.Context, cat *Category) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO categories (parent_id, name, slug, description, icon, sort_order, status, visibility, posting_policy, review_policy)
		VALUES (NULLIF($1,0), $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`,
		cat.ParentID, cat.Name, cat.Slug, cat.Description, cat.Icon, cat.SortOrder,
		cat.Status, cat.Visibility, cat.PostingPolicy, cat.ReviewPolicy).Scan(&id)
	if err != nil && (err.Error() == `ERROR: duplicate key value violates unique constraint "categories_slug_key" (SQLSTATE 23505)`) {
		return 0, apperr.Conflict("CONFLICT", "版块 slug 已存在")
	}
	return id, err
}

// Update 更新版块。
func (r *Repo) Update(ctx context.Context, id int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	set := "updated_at = now_utc()"
	args := []any{}
	i := 1
	for col, val := range fields {
		set += ", " + col + " = $" + itoa(i)
		args = append(args, val)
		i++
	}
	args = append(args, id)
	tag, err := r.pool.Exec(ctx, `UPDATE categories SET `+set+` WHERE id = $`+itoa(i), args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("版块不存在")
	}
	return nil
}

// SoftDelete 软删除版块。
func (r *Repo) SoftDelete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE categories SET deleted_at = now_utc(), updated_at = now_utc() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("版块不存在")
	}
	return nil
}

// Moderators 版主列表。
type Moderator struct {
	PublicID    string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarKey   string `json:"avatar_key"`
}

// ListModerators 版主列表。
func (r *Repo) ListModerators(ctx context.Context, categoryID int64) ([]Moderator, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT u.public_id::text, u.username, COALESCE(p.display_name, u.username), p.avatar_key
		FROM category_moderators cm
		JOIN users u ON u.id = cm.user_id AND u.deleted_at IS NULL
		JOIN user_profiles p ON p.user_id = u.id
		WHERE cm.category_id = $1 ORDER BY cm.created_at`, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Moderator
	for rows.Next() {
		var m Moderator
		if err := rows.Scan(&m.PublicID, &m.Username, &m.DisplayName, &m.AvatarKey); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AddModerator 添加版主（user_roles 版块范围角色 + category_moderators 表）。
func (r *Repo) AddModerator(ctx context.Context, tx pgx.Tx, categoryID, userID int64) error {
	if _, err := tx.Exec(ctx,
		`INSERT INTO category_moderators (category_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		categoryID, userID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id, scope_type, scope_id)
		SELECT $1, id, 'category', $2 FROM roles WHERE code = 'moderator'
		ON CONFLICT DO NOTHING`, userID, categoryID)
	return err
}

// RemoveModerator 移除版主。
func (r *Repo) RemoveModerator(ctx context.Context, tx pgx.Tx, categoryID, userID int64) error {
	if _, err := tx.Exec(ctx,
		`DELETE FROM category_moderators WHERE category_id = $1 AND user_id = $2`, categoryID, userID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		DELETE FROM user_roles
		WHERE user_id = $1 AND scope_type = 'category' AND scope_id = $2
		  AND role_id = (SELECT id FROM roles WHERE code = 'moderator')`, userID, categoryID)
	return err
}

// RecountPost 原子更新版块帖子计数。
func (r *Repo) RecountPost(ctx context.Context, categoryID int64, delta int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE categories SET post_count = GREATEST(0, post_count + $2) WHERE id = $1`, categoryID, delta)
	return err
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}

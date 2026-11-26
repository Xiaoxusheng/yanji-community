// Package users 用户资料、隐私设置与公开主页。
package users

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/shared/apperr"
)

// Repo 用户数据访问。
type Repo struct{ pool *pgxpool.Pool }

// NewRepo 构造。
func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// PublicProfile 公开资料。
type PublicProfile struct {
	PublicID          string    `json:"id"`
	Username          string    `json:"username"`
	DisplayName       string    `json:"display_name"`
	AvatarKey         string    `json:"avatar_key"`
	Bio               string    `json:"bio"`
	Website           string    `json:"website"`
	Location          string    `json:"location"`
	ProfileVisibility string    `json:"profile_visibility"`
	Restricted        bool      `json:"restricted,omitempty"`
	IsFollowing       bool      `json:"is_following"`
	IsBlocked         bool      `json:"is_blocked"`
	CreatedAt         time.Time `json:"created_at"`
	PostCount         int64     `json:"post_count"`
	CommentCount      int64     `json:"comment_count"`
	FollowerCount     int64     `json:"follower_count"`
	FollowingCount    int64     `json:"following_count"`
	ShowFollowers     bool      `json:"show_followers"`
}

// GetPublicProfile 查询公开资料（含计数、关注/拉黑状态与隐私可见性）。
func (r *Repo) GetPublicProfile(ctx context.Context, username string, viewerID int64) (*PublicProfile, error) {
	var p PublicProfile
	var targetID int64
	err := r.pool.QueryRow(ctx, `
		SELECT u.id, u.public_id::text, u.username, COALESCE(p.display_name, u.username), p.avatar_key, p.bio,
		       p.website, p.location, p.profile_visibility, u.created_at,
		       (SELECT COUNT(*) FROM posts WHERE author_id = u.id AND status IN ('published', 'locked') AND deleted_at IS NULL),
		       (SELECT COUNT(*) FROM comments WHERE author_id = u.id AND status = 'published' AND deleted_at IS NULL),
		       (SELECT COUNT(*) FROM user_follows WHERE following_id = u.id),
		       (SELECT COUNT(*) FROM user_follows WHERE follower_id = u.id),
		       p.show_followers
		FROM users u
		JOIN user_profiles p ON p.user_id = u.id
		WHERE u.username = $1 AND u.deleted_at IS NULL AND u.status IN ('active','pending')`,
		username).Scan(
		&targetID, &p.PublicID, &p.Username, &p.DisplayName, &p.AvatarKey, &p.Bio,
		&p.Website, &p.Location, &p.ProfileVisibility, &p.CreatedAt,
		&p.PostCount, &p.CommentCount, &p.FollowerCount, &p.FollowingCount, &p.ShowFollowers)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("用户不存在")
	}
	if err != nil {
		return nil, err
	}
	if viewerID > 0 && viewerID != targetID {
		_ = r.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM user_follows WHERE follower_id = $1 AND following_id = $2)`,
			viewerID, targetID).Scan(&p.IsFollowing)
		_ = r.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM user_blocks WHERE blocker_id = $1 AND blocked_id = $2)`,
			viewerID, targetID).Scan(&p.IsBlocked)
	}
	// 主页隐私控制：private 仅自己可见详情；followers 仅自己或已关注可见详情
	if viewerID != targetID {
		if p.ProfileVisibility == "private" || (p.ProfileVisibility == "followers" && !p.IsFollowing) {
			p.Restricted = true
			p.Bio = ""
			p.Website = ""
			p.Location = ""
		}
	}
	return &p, nil
}

// InternalIDByUsername 由用户名取内部 ID。
func (r *Repo) InternalIDByUsername(ctx context.Context, username string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM users WHERE username = $1 AND deleted_at IS NULL`, username).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, apperr.NotFound("用户不存在")
	}
	return id, err
}

// ProfileSetting 当前用户的资料与设置。
type ProfileSetting struct {
	DisplayName         string `json:"display_name"`
	AvatarKey           string `json:"avatar_key"`
	Bio                 string `json:"bio"`
	Website             string `json:"website"`
	Location            string `json:"location"`
	ProfileVisibility   string `json:"profile_visibility"`
	ShowFollowers       bool   `json:"show_followers"`
	NotifyEmail         bool   `json:"notify_email"`
	NotifyInapp         bool   `json:"notify_inapp"`
	AllowDirectMessages bool   `json:"allow_direct_messages"`
}

// GetMyProfile 查询本人资料。
func (r *Repo) GetMyProfile(ctx context.Context, userID int64) (*ProfileSetting, error) {
	var p ProfileSetting
	err := r.pool.QueryRow(ctx, `
		SELECT p.display_name, p.avatar_key, p.bio, p.website, p.location,
		       p.profile_visibility, p.show_followers, p.notify_email, p.notify_inapp, p.allow_direct_messages
		FROM user_profiles p WHERE p.user_id = $1`, userID).Scan(
		&p.DisplayName, &p.AvatarKey, &p.Bio, &p.Website, &p.Location,
		&p.ProfileVisibility, &p.ShowFollowers, &p.NotifyEmail, &p.NotifyInapp, &p.AllowDirectMessages)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdateProfile 更新资料字段（仅更新传入的非空字段集合）。
func (r *Repo) UpdateProfile(ctx context.Context, userID int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	set := "updated_at = now_utc()"
	args := []any{userID}
	i := 1
	for col, val := range fields {
		i++
		set += ", " + col + " = $" + itoa(i)
		args = append(args, val)
	}
	_, err := r.pool.Exec(ctx, `UPDATE user_profiles SET `+set+` WHERE user_id = $1`, args...)
	return err
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}

// FollowerRow 粉丝/关注行。
type FollowerRow struct {
	PublicID    string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	AvatarKey   string    `json:"avatar_key"`
	Bio         string    `json:"bio"`
	CreatedAt   time.Time `json:"followed_at"`
}

// ListFollowers 粉丝列表（尊重对方主页隐私：非 public 不返回 bio）。
func (r *Repo) ListFollowers(ctx context.Context, userID int64, limit, offset int) ([]FollowerRow, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_follows WHERE following_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT u.public_id::text, u.username, COALESCE(p.display_name, u.username), p.avatar_key,
		       CASE WHEN p.profile_visibility = 'public' THEN p.bio ELSE '' END,
		       f.created_at
		FROM user_follows f
		JOIN users u ON u.id = f.follower_id AND u.deleted_at IS NULL
		JOIN user_profiles p ON p.user_id = u.id
		WHERE f.following_id = $1
		ORDER BY f.created_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []FollowerRow
	for rows.Next() {
		var fr FollowerRow
		if err := rows.Scan(&fr.PublicID, &fr.Username, &fr.DisplayName, &fr.AvatarKey, &fr.Bio, &fr.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, fr)
	}
	return out, total, rows.Err()
}

// ListFollowing 关注列表（尊重对方主页隐私：非 public 不返回 bio）。
func (r *Repo) ListFollowing(ctx context.Context, userID int64, limit, offset int) ([]FollowerRow, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_follows WHERE follower_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT u.public_id::text, u.username, COALESCE(p.display_name, u.username), p.avatar_key,
		       CASE WHEN p.profile_visibility = 'public' THEN p.bio ELSE '' END,
		       f.created_at
		FROM user_follows f
		JOIN users u ON u.id = f.following_id AND u.deleted_at IS NULL
		JOIN user_profiles p ON p.user_id = u.id
		WHERE f.follower_id = $1
		ORDER BY f.created_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []FollowerRow
	for rows.Next() {
		var fr FollowerRow
		if err := rows.Scan(&fr.PublicID, &fr.Username, &fr.DisplayName, &fr.AvatarKey, &fr.Bio, &fr.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, fr)
	}
	return out, total, rows.Err()
}

// FollowedCategoryRow 关注的版块行。
type FollowedCategoryRow struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	PostCount   int64     `json:"post_count"`
	FollowedAt  time.Time `json:"followed_at"`
}

// ListFollowedCategories 我关注的版块列表。
func (r *Repo) ListFollowedCategories(ctx context.Context, userID int64, limit, offset int) ([]FollowedCategoryRow, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM category_follows WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.name, c.slug, c.description, c.post_count, cf.created_at
		FROM category_follows cf
		JOIN categories c ON c.id = cf.category_id AND c.deleted_at IS NULL
		WHERE cf.user_id = $1
		ORDER BY cf.created_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []FollowedCategoryRow
	for rows.Next() {
		var row FollowedCategoryRow
		if err := rows.Scan(&row.ID, &row.Name, &row.Slug, &row.Description, &row.PostCount, &row.FollowedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}

// FollowedTagRow 关注的标签行。
type FollowedTagRow struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Slug       string    `json:"slug"`
	PostCount  int64     `json:"post_count"`
	FollowedAt time.Time `json:"followed_at"`
}

// ListFollowedTags 我关注的标签列表。
func (r *Repo) ListFollowedTags(ctx context.Context, userID int64, limit, offset int) ([]FollowedTagRow, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM tag_follows WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.name, t.slug, t.post_count, tf.created_at
		FROM tag_follows tf
		JOIN tags t ON t.id = tf.tag_id AND t.status = 'active'
		WHERE tf.user_id = $1
		ORDER BY tf.created_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []FollowedTagRow
	for rows.Next() {
		var row FollowedTagRow
		if err := rows.Scan(&row.ID, &row.Name, &row.Slug, &row.PostCount, &row.FollowedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}

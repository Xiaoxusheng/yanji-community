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
	PublicID       string    `json:"id"`
	Username       string    `json:"username"`
	DisplayName    string    `json:"display_name"`
	AvatarKey      string    `json:"avatar_key"`
	Bio            string    `json:"bio"`
	Website        string    `json:"website"`
	Location       string    `json:"location"`
	CreatedAt      time.Time `json:"created_at"`
	PostCount      int64     `json:"post_count"`
	CommentCount   int64     `json:"comment_count"`
	FollowerCount  int64     `json:"follower_count"`
	FollowingCount int64     `json:"following_count"`
	ShowFollowers  bool      `json:"show_followers"`
}

// GetPublicProfile 查询公开资料（含计数）。
func (r *Repo) GetPublicProfile(ctx context.Context, username string) (*PublicProfile, error) {
	var p PublicProfile
	err := r.pool.QueryRow(ctx, `
		SELECT u.public_id::text, u.username, COALESCE(p.display_name, u.username), p.avatar_key, p.bio,
		       p.website, p.location, u.created_at,
		       (SELECT COUNT(*) FROM posts WHERE author_id = u.id AND status = 'published' AND deleted_at IS NULL),
		       (SELECT COUNT(*) FROM comments WHERE author_id = u.id AND status = 'published' AND deleted_at IS NULL),
		       (SELECT COUNT(*) FROM user_follows WHERE following_id = u.id),
		       (SELECT COUNT(*) FROM user_follows WHERE follower_id = u.id),
		       p.show_followers
		FROM users u
		JOIN user_profiles p ON p.user_id = u.id
		WHERE u.username = $1 AND u.deleted_at IS NULL AND u.status IN ('active','pending')`,
		username).Scan(
		&p.PublicID, &p.Username, &p.DisplayName, &p.AvatarKey, &p.Bio,
		&p.Website, &p.Location, &p.CreatedAt,
		&p.PostCount, &p.CommentCount, &p.FollowerCount, &p.FollowingCount, &p.ShowFollowers)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("用户不存在")
	}
	if err != nil {
		return nil, err
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
	DisplayName       string `json:"display_name"`
	AvatarKey         string `json:"avatar_key"`
	Bio               string `json:"bio"`
	Website           string `json:"website"`
	Location          string `json:"location"`
	ProfileVisibility string `json:"profile_visibility"`
	ShowFollowers     bool   `json:"show_followers"`
	NotifyEmail       bool   `json:"notify_email"`
	NotifyInapp       bool   `json:"notify_inapp"`
}

// GetMyProfile 查询本人资料。
func (r *Repo) GetMyProfile(ctx context.Context, userID int64) (*ProfileSetting, error) {
	var p ProfileSetting
	err := r.pool.QueryRow(ctx, `
		SELECT p.display_name, p.avatar_key, p.bio, p.website, p.location,
		       p.profile_visibility, p.show_followers, p.notify_email, p.notify_inapp
		FROM user_profiles p WHERE p.user_id = $1`, userID).Scan(
		&p.DisplayName, &p.AvatarKey, &p.Bio, &p.Website, &p.Location,
		&p.ProfileVisibility, &p.ShowFollowers, &p.NotifyEmail, &p.NotifyInapp)
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

// ListFollowers 粉丝列表。
func (r *Repo) ListFollowers(ctx context.Context, userID int64, limit, offset int) ([]FollowerRow, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_follows WHERE following_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT u.public_id::text, u.username, COALESCE(p.display_name, u.username), p.avatar_key, p.bio, f.created_at
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

// ListFollowing 关注列表。
func (r *Repo) ListFollowing(ctx context.Context, userID int64, limit, offset int) ([]FollowerRow, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_follows WHERE follower_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT u.public_id::text, u.username, COALESCE(p.display_name, u.username), p.avatar_key, p.bio, f.created_at
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

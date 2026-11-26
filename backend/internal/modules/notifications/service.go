package notifications

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/shared/apperr"
	"github.com/yanji-community/backend/internal/shared/pagination"
)

// Notification 通知列表项。
type Notification struct {
	ID           int64      `json:"id"`
	Type         string     `json:"type"`
	ResourceType string     `json:"resource_type"`
	ResourceID   int64      `json:"resource_id"`
	ResourcePID  string     `json:"resource_public_id"`
	Title        string     `json:"title"`
	Body         string     `json:"body"`
	ActorName    string     `json:"actor_name"`
	ActorAvatar  string     `json:"actor_avatar"`
	ReadAt       *time.Time `json:"read_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

// Service 通知服务。
type Service struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// New 构造。
func New(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{pool: pool, logger: logger}
}

// List 通知列表（unread=true 仅未读；type 筛选）。
func (s *Service) List(ctx context.Context, userID int64, pq pagination.Query, unread bool, ntype string) ([]Notification, int64, error) {
	where := "n.recipient_id = $1"
	args := []any{userID}
	if unread {
		where += " AND n.read_at IS NULL"
	}
	if ntype != "" {
		args = append(args, ntype)
		where += " AND n.type = $" + itoaArg(len(args))
	}

	var total int64
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications n WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pq.PageSize, pq.Offset)
	rows, err := s.pool.Query(ctx, `
		SELECT n.id, n.type, n.resource_type, n.resource_id,
		       COALESCE(CASE n.resource_type
		         WHEN 'post' THEN (SELECT p.public_id::text FROM posts p WHERE p.id = n.resource_id)
		         WHEN 'comment' THEN (SELECT c.public_id::text FROM comments c WHERE c.id = n.resource_id)
		         ELSE ''
		       END, ''),
		       n.title, n.body, COALESCE(a.username, ''), COALESCE(ap.avatar_key, ''),
		       n.read_at, n.created_at
		FROM notifications n
		LEFT JOIN users a ON a.id = n.actor_id
		LEFT JOIN user_profiles ap ON ap.user_id = a.id
		WHERE `+where+`
		ORDER BY n.created_at DESC
		LIMIT $`+itoaArg(len(args)-1)+` OFFSET $`+itoaArg(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.Type, &n.ResourceType, &n.ResourceID, &n.ResourcePID,
			&n.Title, &n.Body, &n.ActorName, &n.ActorAvatar, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, n)
	}
	return out, total, rows.Err()
}

func itoaArg(i int) string { return strconv.Itoa(i) }

// UnreadCount 未读数。
func (s *Service) UnreadCount(ctx context.Context, userID int64) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE recipient_id = $1 AND read_at IS NULL`, userID).Scan(&n)
	return n, err
}

// MarkRead 单条已读。
func (s *Service) MarkRead(ctx context.Context, userID, notificationID int64) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE notifications SET read_at = now_utc() WHERE id = $1 AND recipient_id = $2 AND read_at IS NULL`,
		notificationID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("通知不存在")
	}
	return nil
}

// MarkAllRead 全部已读。
func (s *Service) MarkAllRead(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE notifications SET read_at = now_utc() WHERE recipient_id = $1 AND read_at IS NULL`, userID)
	return err
}

// Delete 删除通知。
func (s *Service) Delete(ctx context.Context, userID, notificationID int64) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM notifications WHERE id = $1 AND recipient_id = $2`, notificationID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("通知不存在")
	}
	return nil
}

// Preferences 通知偏好（存于 user_profiles）。
func (s *Service) Preferences(ctx context.Context, userID int64) (map[string]bool, error) {
	var email, inapp bool
	err := s.pool.QueryRow(ctx,
		`SELECT notify_email, notify_inapp FROM user_profiles WHERE user_id = $1`, userID).
		Scan(&email, &inapp)
	if errors.Is(err, pgx.ErrNoRows) {
		return map[string]bool{"notify_email": true, "notify_inapp": true}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]bool{"notify_email": email, "notify_inapp": inapp}, nil
}

// UpdatePreferences 更新通知偏好。
func (s *Service) UpdatePreferences(ctx context.Context, userID int64, email, inapp *bool) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE user_profiles SET
		  notify_email = COALESCE($2, notify_email),
		  notify_inapp = COALESCE($3, notify_inapp),
		  updated_at = now_utc()
		WHERE user_id = $1`, userID, email, inapp)
	return err
}

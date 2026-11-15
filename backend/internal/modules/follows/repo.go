// Package follows 关注（用户/版块/标签）与拉黑。
// 幂等：依赖数据库唯一约束 + ON CONFLICT DO NOTHING，重复请求不产生重复记录。
package follows

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/shared/apperr"
)

// Repo 关注数据访问。
type Repo struct{ pool *pgxpool.Pool }

// NewRepo 构造。
func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// followResult 表示是否发生了新插入（用于首次关注通知）。
type followResult struct{ inserted bool }

func followQuery(ctx context.Context, pool *pgxpool.Pool, query string, args ...any) (followResult, error) {
	tag, err := pool.Exec(ctx, query, args...)
	if err != nil {
		// 自关注/自拉黑由 CHECK 约束拒绝
		if err.Error() == "ERROR: new row for relation \"user_follows\" violates check constraint \"user_follows_no_self\" (SQLSTATE 23514)" {
			return followResult{}, apperr.BadRequest("VALIDATION_ERROR", "不能关注自己")
		}
		return followResult{}, err
	}
	return followResult{inserted: tag.RowsAffected() > 0}, nil
}

// ResolveUserByPublicID 由 public_id 取内部 ID。
func (r *Repo) ResolveUserByPublicID(ctx context.Context, publicID string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM users WHERE public_id = $1 AND deleted_at IS NULL`, publicID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, apperr.NotFound("用户不存在")
	}
	return id, err
}

// FollowUser 关注用户，返回是否为新关注。
func (r *Repo) FollowUser(ctx context.Context, followerID, targetID int64) (bool, error) {
	if followerID == targetID {
		return false, apperr.BadRequest("VALIDATION_ERROR", "不能关注自己")
	}
	res, err := followQuery(ctx, r.pool,
		`INSERT INTO user_follows (follower_id, following_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		followerID, targetID)
	return res.inserted, err
}

// UnfollowUser 取消关注。
func (r *Repo) UnfollowUser(ctx context.Context, followerID, targetID int64) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM user_follows WHERE follower_id = $1 AND following_id = $2`, followerID, targetID)
	return err
}

// BlockUser 拉黑用户（双向屏蔽内容流）。
func (r *Repo) BlockUser(ctx context.Context, blockerID, targetID int64) error {
	if blockerID == targetID {
		return apperr.BadRequest("VALIDATION_ERROR", "不能拉黑自己")
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO user_blocks (blocker_id, blocked_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		blockerID, targetID)
	return err
}

// UnblockUser 取消拉黑。
func (r *Repo) UnblockUser(ctx context.Context, blockerID, targetID int64) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM user_blocks WHERE blocker_id = $1 AND blocked_id = $2`, blockerID, targetID)
	return err
}

// BlockedIDs 我拉黑的与拉黑我的用户 ID 集合（内容流过滤）。
func (r *Repo) BlockedIDs(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT blocked_id FROM user_blocks WHERE blocker_id = $1
		UNION
		SELECT blocker_id FROM user_blocks WHERE blocked_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// IsBlockedPair 双向是否拉黑。
func (r *Repo) IsBlockedPair(ctx context.Context, a, b int64) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_blocks WHERE (blocker_id = $1 AND blocked_id = $2)
			                            OR (blocker_id = $2 AND blocked_id = $1))`, a, b).Scan(&exists)
	return exists, err
}

// ResolveCategoryBySlug 由 slug 取版块 ID。
func (r *Repo) ResolveCategoryBySlug(ctx context.Context, slug string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM categories WHERE slug = $1 AND deleted_at IS NULL`, slug).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, apperr.NotFound("版块不存在")
	}
	return id, err
}

// FollowCategory 关注版块。
func (r *Repo) FollowCategory(ctx context.Context, userID, categoryID int64) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO category_follows (category_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		categoryID, userID)
	return err
}

// UnfollowCategory 取消关注版块。
func (r *Repo) UnfollowCategory(ctx context.Context, userID, categoryID int64) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM category_follows WHERE category_id = $1 AND user_id = $2`, categoryID, userID)
	return err
}

// ResolveTagBySlug 由 slug 取标签 ID。
func (r *Repo) ResolveTagBySlug(ctx context.Context, slug string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM tags WHERE slug = $1 AND status = 'active'`, slug).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, apperr.NotFound("标签不存在")
	}
	return id, err
}

// FollowTag 关注标签。
func (r *Repo) FollowTag(ctx context.Context, userID, tagID int64) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO tag_follows (tag_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, tagID, userID)
	return err
}

// UnfollowTag 取消关注标签。
func (r *Repo) UnfollowTag(ctx context.Context, userID, tagID int64) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM tag_follows WHERE tag_id = $1 AND user_id = $2`, tagID, userID)
	return err
}

// FollowedCategoryIDs 用户关注的版块集合。
func (r *Repo) FollowedCategoryIDs(ctx context.Context, userID int64) ([]int64, error) {
	return r.intIDs(ctx, `SELECT category_id FROM category_follows WHERE user_id = $1`, userID)
}

// FollowedTagIDs 用户关注的标签集合。
func (r *Repo) FollowedTagIDs(ctx context.Context, userID int64) ([]int64, error) {
	return r.intIDs(ctx, `SELECT tag_id FROM tag_follows WHERE user_id = $1`, userID)
}

// FollowedUserIDs 用户关注的作者集合。
func (r *Repo) FollowedUserIDs(ctx context.Context, userID int64) ([]int64, error) {
	return r.intIDs(ctx, `SELECT following_id FROM user_follows WHERE follower_id = $1`, userID)
}

func (r *Repo) intIDs(ctx context.Context, query string, userID int64) ([]int64, error) {
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

package posts

import (
	"context"

	"github.com/yanji-community/backend/internal/modules/moderation"
	"github.com/yanji-community/backend/internal/modules/notifications"
	"github.com/yanji-community/backend/internal/shared/apperr"
	"github.com/yanji-community/backend/internal/shared/rbac"
)

// Moderate 版主/管理员内容管理动作（服务端状态机 + 版块范围校验 + 审计 + 通知）。
func (s *Service) Moderate(ctx context.Context, actor *Viewer, actorID int64, requestID string, publicID string, req ModerateRequest) error {
	row, err := s.repo.GetByPublicID(ctx, publicID, 0)
	if err != nil {
		return err
	}
	if !actor.CanModerateCategory(row.CategoryID) {
		return apperr.Forbidden("只能管理授权版块内的内容")
	}

	fields := map[string]any{}
	statusChange := ""
	reason := req.Reason
	notify := true
	switch req.Action {
	case "approve":
		if row.Status != StatusPendingReview {
			return apperr.Conflict("CONFLICT", "仅待审核内容可通过")
		}
		statusChange = StatusPublished
	case "reject":
		if row.Status != StatusPendingReview {
			return apperr.Conflict("CONFLICT", "仅待审核内容可驳回")
		}
		if reason == "" {
			return apperr.BadRequest("VALIDATION_ERROR", "驳回必须填写原因")
		}
		statusChange = StatusRejected
		fields["reject_reason"] = reason
	case "hide":
		if row.Status != StatusPublished && row.Status != StatusLocked {
			return apperr.Conflict("CONFLICT", "仅已发布内容可隐藏")
		}
		statusChange = StatusHidden
		fields["is_pinned"] = false
		fields["is_featured"] = false
	case "restore":
		if row.Status != StatusHidden {
			return apperr.Conflict("CONFLICT", "仅隐藏内容可恢复")
		}
		statusChange = StatusPublished
	case "lock":
		if row.Status != StatusPublished {
			return apperr.Conflict("CONFLICT", "仅已发布内容可锁定")
		}
		statusChange = StatusLocked
	case "unlock":
		if row.Status != StatusLocked {
			return apperr.Conflict("CONFLICT", "仅锁定内容可解锁")
		}
		statusChange = StatusPublished
	case "pin":
		if row.Status != StatusPublished {
			return apperr.Conflict("CONFLICT", "仅已发布内容可置顶")
		}
		fields["is_pinned"] = true
		notify = false
	case "unpin":
		fields["is_pinned"] = false
		notify = false
	case "feature":
		if row.Status != StatusPublished {
			return apperr.Conflict("CONFLICT", "仅已发布内容可加精")
		}
		fields["is_featured"] = true
		notify = false
	case "unfeature":
		fields["is_featured"] = false
		notify = false
	case "move_category":
		if req.CategorySlug == "" {
			return apperr.BadRequest("VALIDATION_ERROR", "请指定目标版块")
		}
		cat, err := s.deps.Cats.GetBySlug(ctx, req.CategorySlug)
		if err != nil {
			return err
		}
		if !actor.CanModerateCategory(cat.ID) {
			return apperr.Forbidden("无权移动到目标版块")
		}
		if cat.ID != row.CategoryID {
			fields["category_id"] = cat.ID
			if row.Status == StatusPublished {
				if _, err := s.deps.Pool.Exec(ctx,
					`UPDATE categories SET post_count = GREATEST(0, post_count-1) WHERE id = $1`, row.CategoryID); err != nil {
					return apperr.Internal(err)
				}
				if _, err := s.deps.Pool.Exec(ctx,
					`UPDATE categories SET post_count = post_count + 1 WHERE id = $1`, cat.ID); err != nil {
					return apperr.Internal(err)
				}
			}
		}
		notify = false
	default:
		return apperr.BadRequest("VALIDATION_ERROR", "不支持的管理动作")
	}

	tx, err := s.deps.Pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)

	if statusChange != "" {
		wasPublished := row.Status == StatusPublished
		willPublish := statusChange == StatusPublished
		if err := s.repo.SetStatusTx(ctx, tx, row.ID, statusChange, reason); err != nil {
			return apperr.Internal(err)
		}
		if wasPublished && !willPublish {
			if _, err := tx.Exec(ctx,
				`UPDATE categories SET post_count = GREATEST(0, post_count - 1) WHERE id = $1`, row.CategoryID); err != nil {
				return apperr.Internal(err)
			}
		}
		if !wasPublished && willPublish {
			if _, err := tx.Exec(ctx,
				`UPDATE categories SET post_count = post_count + 1 WHERE id = $1`, row.CategoryID); err != nil {
				return apperr.Internal(err)
			}
		}
	}
	if len(fields) > 0 {
		if err := s.repo.ModerateTx(ctx, tx, row.ID, fields); err != nil {
			return apperr.Internal(err)
		}
	}
	// 审计（事务内）
	s.deps.Audit.LogInTx(ctx, tx, moderation.Detail{
		Action: "post." + req.Action, TargetType: "post", TargetID: row.ID,
		ActorID: actorID, RequestID: requestID, Note: reason,
	})
	// 作者通知
	if notify && row.AuthorID != actorID {
		t := truncate(row.Title, 40)
		title, body := "你的帖子「"+t+"」已被管理", ""
		switch req.Action {
		case "approve":
			title = "你的帖子「" + t + "」已通过审核"
		case "reject":
			title = "你的帖子「" + t + "」未通过审核"
			body = "原因：" + reason
		case "hide":
			title = "你的帖子「" + t + "」已被隐藏"
			body = "原因：" + reason
		case "lock":
			title = "你的帖子「" + t + "」已被锁定"
		case "restore":
			title = "你的帖子「" + t + "」已恢复"
		}
		if err := notifications.EmitTx(ctx, tx, notifications.Payload{
			RecipientID: row.AuthorID, ActorID: actorID, Type: notifications.TypeModeration,
			ResourceType: "post", ResourceID: row.ID, Title: title, Body: body,
			DedupeKey: "postmod:" + req.Action + ":" + itoa(row.ID) + ":" + itoa(actorID),
		}); err != nil {
			return apperr.Internal(err)
		}
	}
	return tx.Commit(ctx)
}

// Feature 精华位管理（featured_items）。
func (s *Service) FeatureItem(ctx context.Context, operatorID int64, publicID string, add bool) error {
	row, err := s.repo.GetByPublicID(ctx, publicID, 0)
	if err != nil {
		return err
	}
	if add {
		_, err = s.deps.Pool.Exec(ctx, `
			INSERT INTO featured_items (item_type, item_id, operator_id) VALUES ('post', $1, $2)
			ON CONFLICT (item_type, item_id, position) DO NOTHING`, row.ID, operatorID)
	} else {
		_, err = s.deps.Pool.Exec(ctx,
			`DELETE FROM featured_items WHERE item_type = 'post' AND item_id = $1`, row.ID)
	}
	return err
}

// AdminList 后台列表。
func (s *Service) AdminList(ctx context.Context, status, categorySlug, q string, limit, offset int) ([]PostItem, int64, error) {
	rows, total, err := s.repo.AdminList(ctx, status, categorySlug, q, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.loadTagsForRows(ctx, rows)
	return items, total, err
}

// StaffViewer 构造观察者上下文（handler 使用）：区分全局管理权限与版块范围授权。
func StaffViewer(rbacSvc *rbac.Service, userID int64) *Viewer {
	v := &Viewer{UserID: userID}
	global, _ := rbacSvc.HasGlobalPermission(context.Background(), userID, "post.moderate")
	v.IsStaff = global
	ids, _ := rbacSvc.ModeratedCategoryIDs(context.Background(), userID)
	v.ModeratedCategories = ids
	return v
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

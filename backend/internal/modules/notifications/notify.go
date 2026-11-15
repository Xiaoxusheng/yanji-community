// Package notifications 通知：业务侧经 Outbox 异步写入，读取侧提供未读数/列表/已读。
package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/yanji-community/backend/internal/shared/outbox"
)

// 通知类型常量。
const (
	TypeComment     = "comment"      // 帖子收到评论
	TypeReply       = "reply"        // 评论收到回复
	TypeMention     = "mention"      // @提及
	TypeLikePost    = "like_post"    // 帖子被点赞
	TypeLikeComment = "like_comment" // 评论被点赞
	TypeFollow      = "follow"       // 新粉丝
	TypeModeration  = "moderation"   // 审核/处罚
	TypeSystem      = "system"       // 系统通知
)

// Payload outbox 事件负载。
type Payload struct {
	RecipientID  int64  `json:"recipient_id"`
	ActorID      int64  `json:"actor_id"`
	Type         string `json:"type"`
	ResourceType string `json:"resource_type"`
	ResourceID   int64  `json:"resource_id"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	DedupeKey    string `json:"dedupe_key,omitempty"`
}

// EmitTx 在业务事务内写入通知事件（与主数据同事务提交，保证可靠）。
// recipient == actor 时不产生自通知；用户关闭站内通知时跳过社交类通知
// （moderation/system 类保留，确保处罚与系统消息不丢失）。
func EmitTx(ctx context.Context, tx pgx.Tx, p Payload) error {
	if p.RecipientID == 0 || p.RecipientID == p.ActorID {
		return nil
	}
	switch p.Type {
	case TypeComment, TypeReply, TypeMention, TypeLikePost, TypeLikeComment, TypeFollow, "message":
		var inapp bool
		if err := tx.QueryRow(ctx,
			`SELECT notify_inapp FROM user_profiles WHERE user_id = $1`, p.RecipientID).Scan(&inapp); err == nil && !inapp {
			return nil
		}
	}
	return outbox.EmitTx(ctx, tx, "notification.created", p.RecipientID, p)
}

// RegisterConsumer 注册 outbox → notifications 消费者（rdb 可空：为空时无实时推送）。
func RegisterConsumer(d *outbox.Dispatcher, pool *pgxpool.Pool, rdb redis.UniversalClient) {
	d.Register("notification.created", func(ctx context.Context, aggregateID int64, raw json.RawMessage) error {
		var p Payload
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("通知事件负载不合法: %w", err)
		}
		if p.RecipientID == 0 || p.Title == "" {
			return errors.New("通知事件缺少 recipient/title")
		}
		tag, err := pool.Exec(ctx, `
			INSERT INTO notifications (recipient_id, actor_id, type, resource_type, resource_id, title, body, dedupe_key)
			VALUES ($1, NULLIF($2, 0), $3, $4, $5, $6, $7, NULLIF($8, ''))
			ON CONFLICT (dedupe_key) DO NOTHING`,
			p.RecipientID, p.ActorID, p.Type, p.ResourceType, p.ResourceID, p.Title, p.Body, p.DedupeKey)
		if err != nil {
			return err
		}
		// 先落库后推送：插入成功才发布实时事件（SSE 仅提醒，断线补拉以数据库为准）
		if tag.RowsAffected() > 0 {
			actor := ""
			if p.ActorID > 0 {
				_ = pool.QueryRow(ctx, `SELECT username FROM users WHERE id = $1`, p.ActorID).Scan(&actor)
			}
			PublishNotification(ctx, rdb, p.RecipientID, SSEEvent{Type: p.Type, Title: p.Title, Actor: actor})
		}
		return nil
	})
}

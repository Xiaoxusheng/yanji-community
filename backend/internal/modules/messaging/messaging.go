// Package messaging 一对一私信：会话（get-or-create）、消息持久化、已读、免打扰、置顶、撤回。
// 红线：先可靠落库后推送；防骚扰（拉黑/私信开关/处罚）在服务端逐条校验。
package messaging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/modules/notifications"
	"github.com/yanji-community/backend/internal/shared/apperr"
	httpx "github.com/yanji-community/backend/internal/shared/httpx"
	"github.com/yanji-community/backend/internal/shared/pagination"
	"github.com/yanji-community/backend/internal/shared/respond"
)

// RecallWindow 撤回时间窗口。
const RecallWindow = 5 * time.Minute

// PairKey 1:1 会话去重键（纯函数，单测覆盖）。
func PairKey(a, b int64) string {
	if a > b {
		a, b = b, a
	}
	return fmt.Sprintf("dm:%d:%d", a, b)
}

// CanRecall 纯函数：是否在撤回窗口内（单测覆盖）。
func CanRecall(createdAt time.Time, now time.Time) bool {
	return now.Sub(createdAt) <= RecallWindow && now.After(createdAt) || now.Equal(createdAt)
}

// Service 私信业务。
type Service struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
	// SSE 推送钩子（由 main 注入 notifications.PublishNotification 闭包，避免依赖 Redis 客户端类型）
	PushEvent func(ctx context.Context, recipientID int64, ev notifications.SSEEvent)
}

// New 构造。
func New(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{pool: pool, logger: logger}
}

func invalid(msg string) error { return apperr.BadRequest("VALIDATION_ERROR", msg) }

// Conversation 会话列表项。
type Conversation struct {
	ID            int64      `json:"id"`
	Other         OtherUser  `json:"other"`
	LastMessage   string     `json:"last_message"`
	LastMessageAt *time.Time `json:"last_message_at"`
	LastSenderMe  bool       `json:"last_sender_me"`
	Unread        int64      `json:"unread"`
	Muted         bool       `json:"muted"`
	Pinned        bool       `json:"pinned"`
	CreatedAt     time.Time  `json:"created_at"`
}

// OtherUser 对话方。
type OtherUser struct {
	PublicID    string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarKey   string `json:"avatar_key"`
}

// GetOrCreate 获取或创建与目标用户的 1:1 会话（防骚扰检查在创建/发送前都要过）。
func (s *Service) GetOrCreate(ctx context.Context, userID int64, targetPublicID string) (*Conversation, error) {
	var targetID int64
	var targetStatus string
	var allowDM bool
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.status, p.allow_direct_messages
		FROM users u JOIN user_profiles p ON p.user_id = u.id
		WHERE u.public_id = $1 AND u.deleted_at IS NULL`, targetPublicID).
		Scan(&targetID, &targetStatus, &allowDM)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("用户不存在")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if targetID == userID {
		return nil, invalid("不能给自己发私信")
	}
	if targetStatus == "banned" || targetStatus == "suspended" {
		return nil, apperr.Conflict("CONFLICT", "对方账号当前无法接收私信")
	}
	// 拉黑双向拒绝
	var blocked bool
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_blocks WHERE (blocker_id = $1 AND blocked_id = $2)
			                            OR (blocker_id = $2 AND blocked_id = $1))`, userID, targetID).Scan(&blocked); err != nil {
		return nil, apperr.Internal(err)
	}
	if blocked {
		return nil, apperr.Forbidden("无法与该用户建立私信（存在拉黑关系）")
	}
	// 对方关闭了陌生人私信
	if !allowDM {
		return nil, apperr.Forbidden("对方已关闭私信")
	}

	key := PairKey(userID, targetID)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	var convID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO conversations (pair_key) VALUES ($1)
		ON CONFLICT (pair_key) DO NOTHING
		RETURNING id`, key).Scan(&convID)
	if errors.Is(err, pgx.ErrNoRows) {
		// 已存在：读取
		if err := tx.QueryRow(ctx,
			`SELECT id FROM conversations WHERE pair_key = $1`, key).Scan(&convID); err != nil {
			return nil, apperr.Internal(err)
		}
	} else if err != nil {
		return nil, apperr.Internal(err)
	}
	// 双方成员（幂等）
	for _, uid := range []int64{userID, targetID} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO conversation_members (conversation_id, user_id) VALUES ($1, $2)
			ON CONFLICT (conversation_id, user_id) DO NOTHING`, convID, uid); err != nil {
			return nil, apperr.Internal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, apperr.Internal(err)
	}
	// 读取自己的成员视图
	me, err := s.memberView(ctx, convID, userID)
	if err != nil {
		return nil, err
	}
	return me, nil
}

// memberView 读取指定用户视角的会话视图。
func (s *Service) memberView(ctx context.Context, convID, userID int64) (*Conversation, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT cm.muted, cm.pinned,
		       ou.id, ou.username, COALESCE(op.display_name, ou.username), op.avatar_key
		FROM conversations c
		JOIN conversation_members cm ON cm.conversation_id = c.id AND cm.user_id = $2
		JOIN LATERAL (
			SELECT u.id, u.username FROM conversation_members om
			JOIN users u ON u.id = om.user_id
			WHERE om.conversation_id = c.id AND om.user_id <> $2 LIMIT 1
		) ou ON TRUE
		JOIN user_profiles op ON op.user_id = ou.id
		WHERE c.id = $1`, convID, userID)
	var c Conversation
	var otherID int64
	if err := row.Scan(&c.Muted, &c.Pinned, &otherID, &c.Other.Username, &c.Other.DisplayName, &c.Other.AvatarKey); err != nil {
		return nil, apperr.Internal(err)
	}
	_ = s.pool.QueryRow(ctx, `SELECT public_id::text FROM users WHERE id = $1`, otherID).Scan(&c.Other.PublicID)
	c.ID = convID
	// 最后一条消息预览 + 未读数
	err := s.pool.QueryRow(ctx, `
		SELECT m.body_text, m.sender_id = $2, m.created_at
		FROM messages m
		WHERE m.conversation_id = $1 AND m.status = 'sent'
		ORDER BY m.id DESC LIMIT 1`, convID, userID).
		Scan(&c.LastMessage, &c.LastSenderMe, &c.LastMessageAt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.Internal(err)
	}
	_ = s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM messages m
		WHERE m.conversation_id = $1 AND m.sender_id <> $2 AND m.status = 'sent'
		  AND m.id > (SELECT cm.last_read_message_id FROM conversation_members cm
		              WHERE cm.conversation_id = $1 AND cm.user_id = $2)`,
		convID, userID).Scan(&c.Unread)
	return &c, nil
}

// ListConversations 会话列表（置顶优先，其次按最后消息时间）。
func (s *Service) ListConversations(ctx context.Context, userID int64, limit, offset int) ([]Conversation, int64, error) {
	var total int64
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM conversation_members WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT cm.conversation_id FROM conversation_members cm
		JOIN conversations c ON c.id = cm.conversation_id
		WHERE cm.user_id = $1
		ORDER BY cm.pinned DESC, COALESCE(c.last_message_at, c.created_at) DESC
		LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, 0, err
		}
		ids = append(ids, id)
	}
	out := []Conversation{}
	for _, id := range ids {
		c, err := s.memberView(ctx, id, userID)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *c)
	}
	return out, total, nil
}

// Message 消息。
type Message struct {
	ID        int64     `json:"id"`
	Sender    OtherUser `json:"sender"`
	Mine      bool      `json:"mine"`
	Body      string    `json:"body_text"`
	ImageKey  string    `json:"image_key,omitempty"`
	ReplyToID int64     `json:"reply_to_id,omitempty"`
	ReplyBody string    `json:"reply_body,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// isMember 校验会话成员。
func (s *Service) isMember(ctx context.Context, convID, userID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id = $1 AND user_id = $2)`,
		convID, userID).Scan(&ok)
	return ok, err
}

// ListMessages 消息分页（按时间倒序分页，前端反转展示）。
func (s *Service) ListMessages(ctx context.Context, userID, convID int64, limit, offset int) ([]Message, int64, error) {
	ok, err := s.isMember(ctx, convID, userID)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	if !ok {
		return nil, 0, apperr.Forbidden("不是该会话的成员")
	}
	var total int64
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM messages WHERE conversation_id = $1`, convID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.sender_id, u.public_id::text, u.username, COALESCE(p.display_name, u.username),
		       p.avatar_key, m.sender_id = $2,
		       CASE WHEN m.status = 'sent' THEN m.body_text ELSE '' END,
		       m.reply_to_id,
		       CASE WHEN r.status = 'sent' THEN COALESCE(r.body_text, '') ELSE '' END,
		       m.status, m.created_at,
		       CASE WHEN m.status = 'sent' THEN COALESCE((SELECT ma.storage_key FROM message_attachments ma WHERE ma.message_id = m.id LIMIT 1), '') ELSE '' END
		FROM messages m
		JOIN users u ON u.id = m.sender_id
		JOIN user_profiles p ON p.user_id = u.id
		LEFT JOIN messages r ON r.id = m.reply_to_id
		WHERE m.conversation_id = $1
		ORDER BY m.id DESC LIMIT $3 OFFSET $4`, convID, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		var senderID int64
		var replyTo *int64
		if err := rows.Scan(&m.ID, &senderID, &m.Sender.PublicID, &m.Sender.Username, &m.Sender.DisplayName,
			&m.Sender.AvatarKey, &m.Mine, &m.Body, &replyTo, &m.ReplyBody, &m.Status, &m.CreatedAt, &m.ImageKey); err != nil {
			return nil, 0, err
		}
		if replyTo != nil {
			m.ReplyToID = *replyTo
		}
		out = append(out, m)
	}
	return out, total, rows.Err()
}

// SendMessage 发送消息（落库 → 更新会话 → Outbox 通知 → SSE 推送）。
func (s *Service) SendMessage(ctx context.Context, senderID int64, convID int64, body string, imageKey string, replyToID int64) (*Message, error) {
	body = trimSpace(body)
	if body == "" && imageKey == "" {
		return nil, invalid("消息内容不能为空")
	}
	// 数据库约束要求 messages.body_text 非空：纯图片消息自动补占位文本
	if body == "" && imageKey != "" {
		body = "[图片]"
	}
	if len([]rune(body)) > 5000 {
		return nil, invalid("消息最长 5000 字")
	}
	ok, err := s.isMember(ctx, convID, senderID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if !ok {
		return nil, apperr.Forbidden("不是该会话的成员")
	}
	// 接收方防骚扰复查（关系可能已变化）
	var otherID int64
	err = s.pool.QueryRow(ctx, `
		SELECT user_id FROM conversation_members WHERE conversation_id = $1 AND user_id <> $2 LIMIT 1`,
		convID, senderID).Scan(&otherID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.Conflict("CONFLICT", "会话缺少接收方")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	var blocked bool
	var otherStatus string
	var allowDM bool
	if err := s.pool.QueryRow(ctx, `
		SELECT
		  EXISTS(SELECT 1 FROM user_blocks WHERE (blocker_id = $1 AND blocked_id = $2) OR (blocker_id = $2 AND blocked_id = $1)),
		  u.status, p.allow_direct_messages
		FROM users u JOIN user_profiles p ON p.user_id = u.id WHERE u.id = $1`,
		otherID, senderID).Scan(&blocked, &otherStatus, &allowDM); err != nil {
		return nil, apperr.Internal(err)
	}
	if blocked {
		return nil, apperr.Forbidden("无法发送：存在拉黑关系")
	}
	if otherStatus == "banned" || otherStatus == "suspended" {
		return nil, apperr.Conflict("CONFLICT", "对方账号当前无法接收私信")
	}
	if !allowDM {
		return nil, apperr.Forbidden("对方已关闭私信")
	}
	// 图片附件必须为发送者本人上传
	if imageKey != "" {
		var owner int64
		err := s.pool.QueryRow(ctx,
			`SELECT uploader_id FROM post_attachments WHERE storage_key = $1`, imageKey).Scan(&owner)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && owner != senderID) {
			return nil, invalid("图片无效或非本人上传")
		}
		if err != nil {
			return nil, apperr.Internal(err)
		}
	}
	// 引用消息必须属于本会话
	if replyToID > 0 {
		var belongs bool
		if err := s.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM messages WHERE id = $1 AND conversation_id = $2)`, replyToID, convID).Scan(&belongs); err != nil {
			return nil, apperr.Internal(err)
		}
		if !belongs {
			return nil, invalid("引用消息不属于该会话")
		}
	}

	var senderPID string
	if err := s.pool.QueryRow(ctx,
		`SELECT public_id::text FROM users WHERE id = $1`, senderID).Scan(&senderPID); err != nil {
		return nil, apperr.Internal(err)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	var msgID int64
	var createdAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO messages (conversation_id, sender_id, body_text, reply_to_id)
		VALUES ($1, $2, $3, NULLIF($4, 0))
		RETURNING id, created_at`, convID, senderID, body, replyToID).Scan(&msgID, &createdAt)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if imageKey != "" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO message_attachments (message_id, uploader_id, storage_key, mime_type, size_bytes)
			SELECT $1, $2, $3,
			       COALESCE(NULLIF(mime_type, ''), 'application/octet-stream'),
			       GREATEST(size_bytes, 1)
			FROM post_attachments WHERE storage_key = $3 AND uploader_id = $2`,
			msgID, senderID, imageKey); err != nil {
			return nil, apperr.Internal(err)
		}
	}
	if _, err := tx.Exec(ctx,
		`UPDATE conversations SET last_message_at = $2, updated_at = now_utc() WHERE id = $1`,
		convID, createdAt); err != nil {
		return nil, apperr.Internal(err)
	}
	// Outbox 通知（与消息同事务，可靠）
	var senderName string
	_ = s.pool.QueryRow(ctx, `SELECT COALESCE(display_name, username) FROM user_profiles up
		JOIN users u ON u.id = up.user_id WHERE u.id = $1`, senderID).Scan(&senderName)
	if err := notifications.EmitTx(ctx, tx, notifications.Payload{
		RecipientID: otherID, ActorID: senderID, Type: "message",
		ResourceType: "user", ResourceID: senderID,
		Title:     senderName + " 给你发来了私信",
		Body:      previewOf(body, 60),
		DedupeKey: "dm:" + itoa(msgID),
	}); err != nil {
		return nil, apperr.Internal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, apperr.Internal(err)
	}
	s.logger.Info("dm sent", "from", senderID, "to", otherID, "conv", convID)
	// 提交成功后再推送实时事件（SSE 仅提醒，断线补拉以数据库为准）
	if s.PushEvent != nil {
		s.PushEvent(ctx, otherID, notifications.SSEEvent{Type: "message", Title: senderName + "： " + previewOf(body, 30)})
	}
	return &Message{
		ID: msgID, Sender: OtherUser{PublicID: senderPID}, Mine: true,
		Body: body, ImageKey: imageKey, ReplyToID: replyToID, Status: "sent", CreatedAt: createdAt,
	}, nil
}

// MarkRead 标记会话已读。
func (s *Service) MarkRead(ctx context.Context, userID, convID int64) error {
	ok, err := s.isMember(ctx, convID, userID)
	if err != nil {
		return apperr.Internal(err)
	}
	if !ok {
		return apperr.Forbidden("不是该会话的成员")
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE conversation_members SET last_read_message_id =
		  COALESCE((SELECT MAX(id) FROM messages WHERE conversation_id = $1), 0)
		WHERE conversation_id = $1 AND user_id = $2`, convID, userID)
	return err
}

// UpdateSettings 免打扰/置顶。
func (s *Service) UpdateSettings(ctx context.Context, userID, convID int64, muted, pinned *bool) error {
	ok, err := s.isMember(ctx, convID, userID)
	if err != nil {
		return apperr.Internal(err)
	}
	if !ok {
		return apperr.Forbidden("不是该会话的成员")
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE conversation_members SET
		  muted = COALESCE($3, muted),
		  pinned = COALESCE($4, pinned)
		WHERE conversation_id = $1 AND user_id = $2`, convID, userID, muted, pinned)
	return err
}

// Recall 撤回自己的消息（5 分钟内；同步删除图片附件记录防止撤回后图片残留泄露）。
func (s *Service) Recall(ctx context.Context, userID, messageID int64) error {
	var senderID, convID int64
	var status string
	var createdAt time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT sender_id, conversation_id, status, created_at FROM messages WHERE id = $1`, messageID).
		Scan(&senderID, &convID, &status, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("消息不存在")
	}
	if err != nil {
		return apperr.Internal(err)
	}
	if senderID != userID {
		return apperr.Forbidden("只能撤回自己的消息")
	}
	if status != "sent" {
		return apperr.Conflict("CONFLICT", "消息已撤回")
	}
	if !CanRecall(createdAt, time.Now().UTC()) {
		return apperr.Conflict("CONFLICT", "超过 5 分钟无法撤回")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		UPDATE messages SET status = 'recalled', recalled_at = now_utc(),
		  body_text = ''
		WHERE id = $1`, messageID); err != nil {
		return apperr.Internal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM message_attachments WHERE message_id = $1`, messageID); err != nil {
		return apperr.Internal(err)
	}
	return tx.Commit(ctx)
}

// UnreadTotal 未读消息总数（免打扰会话不计入角标）。
func (s *Service) UnreadTotal(ctx context.Context, userID int64) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM messages m
		JOIN conversation_members cm ON cm.conversation_id = m.conversation_id AND cm.user_id = $1
		WHERE m.sender_id <> $1 AND m.status = 'sent'
		  AND m.id > cm.last_read_message_id
		  AND cm.muted = FALSE`, userID).Scan(&n)
	return n, err
}

// GetByID 会话校验存在（HTTP 用）。
func (s *Service) GetByID(ctx context.Context, userID, convID int64) (*Conversation, error) {
	ok, err := s.isMember(ctx, convID, userID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if !ok {
		return nil, apperr.NotFound("会话不存在")
	}
	return s.memberView(ctx, convID, userID)
}

func previewOf(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ---- HTTP ----

// Handler 私信 HTTP 层。
type Handler struct{ svc *Service }

// NewHandler 构造。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register 注册路由（禁言/封禁用户禁止新建会话或发送消息，并全量纳入 dmRl 限流）。
func (h *Handler) Register(rg *gin.RouterGroup, dmRl gin.HandlerFunc) {
	me := rg.Group("")
	me.Use(httpx.RequireAuth(), httpx.RequireVerified(), httpx.NoStore())
	{
		me.GET("/me/conversations", h.List)
		me.GET("/me/conversations/unread-count", h.UnreadCount)
		me.POST("/conversations", dmRl, httpx.RequireCanComment(), h.Open)
		msgs := me.Group("/conversations/:id")
		msgs.Use(dmRl)
		{
			msgs.GET("/messages", h.Messages)
			msgs.POST("/messages", httpx.RequireCanComment(), h.Send)
			msgs.POST("/read", h.MarkRead)
			msgs.PATCH("/settings", h.Settings)
		}
		me.DELETE("/messages/:id", dmRl, h.Recall)
	}
}

func parseConvID(c *gin.Context) (int64, error) {
	var id int64
	if _, err := fmtSscan(c.Param("id"), &id); err != nil || id <= 0 {
		return 0, invalid("会话 ID 不合法")
	}
	return id, nil
}

func fmtSscan(s string, v *int64) (int, error) {
	n, err := parseI64(s)
	if err != nil {
		return 0, err
	}
	*v = n
	return 1, nil
}

func parseI64(s string) (int64, error) {
	var n int64
	if s == "" {
		return 0, errors.New("empty")
	}
	neg := false
	for i, ch := range s {
		if i == 0 && ch == '-' {
			neg = true
			continue
		}
		if ch < '0' || ch > '9' {
			return 0, errors.New("bad")
		}
		n = n*10 + int64(ch-'0')
	}
	if neg {
		n = -n
	}
	return n, nil
}

// List GET /me/conversations
func (h *Handler) List(c *gin.Context) {
	u := httpx.CurrentUser(c)
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	items, total, err := h.svc.ListConversations(c.Request.Context(), u.ID, pq.PageSize, pq.Offset)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// UnreadCount GET /me/conversations/unread-count
func (h *Handler) UnreadCount(c *gin.Context) {
	u := httpx.CurrentUser(c)
	n, err := h.svc.UnreadTotal(c.Request.Context(), u.ID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"unread": n})
}

type openReq struct {
	TargetID string `json:"target_id"`
}

// Open POST /conversations
func (h *Handler) Open(c *gin.Context) {
	u := httpx.CurrentUser(c)
	var req openReq
	if err := c.ShouldBindJSON(&req); err != nil || req.TargetID == "" {
		respond.Fail(c, invalid("请指定私信对象"))
		return
	}
	conv, err := h.svc.GetOrCreate(c.Request.Context(), u.ID, req.TargetID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"conversation": conv})
}

// Messages GET /conversations/:id/messages
func (h *Handler) Messages(c *gin.Context) {
	u := httpx.CurrentUser(c)
	convID, err := parseConvID(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	items, total, err := h.svc.ListMessages(c.Request.Context(), u.ID, convID, pq.PageSize, pq.Offset)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

type sendReq struct {
	Body      string `json:"body_text"`
	ImageKey  string `json:"image_key"`
	ReplyToID int64  `json:"reply_to_id"`
}

// Send POST /conversations/:id/messages
func (h *Handler) Send(c *gin.Context) {
	u := httpx.CurrentUser(c)
	convID, err := parseConvID(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	var req sendReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, invalid("请求体格式不正确"))
		return
	}
	msg, err := h.svc.SendMessage(c.Request.Context(), u.ID, convID, req.Body, req.ImageKey, req.ReplyToID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.Created(c, gin.H{"message": msg})
}

// MarkRead POST /conversations/:id/read
func (h *Handler) MarkRead(c *gin.Context) {
	u := httpx.CurrentUser(c)
	convID, err := parseConvID(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if err := h.svc.MarkRead(c.Request.Context(), u.ID, convID); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "已读"})
}

type convSettingsReq struct {
	Muted  *bool `json:"muted"`
	Pinned *bool `json:"pinned"`
}

// Settings PATCH /conversations/:id/settings
func (h *Handler) Settings(c *gin.Context) {
	u := httpx.CurrentUser(c)
	convID, err := parseConvID(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	var req convSettingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, invalid("请求体格式不正确"))
		return
	}
	if err := h.svc.UpdateSettings(c.Request.Context(), u.ID, convID, req.Muted, req.Pinned); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "设置已保存"})
}

// Recall DELETE /messages/:id
func (h *Handler) Recall(c *gin.Context) {
	u := httpx.CurrentUser(c)
	msgID, err := parseI64(c.Param("id"))
	if err != nil || msgID <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	if err := h.svc.Recall(c.Request.Context(), u.ID, msgID); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "消息已撤回"})
}

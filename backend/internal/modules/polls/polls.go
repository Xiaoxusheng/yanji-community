// Package polls 投票帖：创建、投票（数据库唯一约束防重 + 单选/多选/改票规则）、结果。
package polls

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/shared/apperr"
	httpx "github.com/yanji-community/backend/internal/shared/httpx"
	"github.com/yanji-community/backend/internal/shared/respond"
)

// Config 创建投票配置。
type Config struct {
	Options        []string   `json:"options"`
	MultipleChoice bool       `json:"multiple_choice"`
	MaxChoices     int        `json:"max_choices"`
	AllowRevote    bool       `json:"allow_revote"`
	EndsAt         *time.Time `json:"ends_at"`
}

// Poll 投票（含结果）。
type Poll struct {
	ID             int64       `json:"id"`
	PostID         int64       `json:"post_id"`
	MultipleChoice bool        `json:"multiple_choice"`
	MaxChoices     int         `json:"max_choices"`
	AllowRevote    bool        `json:"allow_revote"`
	EndsAt         *time.Time  `json:"ends_at"`
	Expired        bool        `json:"expired"`
	TotalVoters    int64       `json:"total_voters"`
	MyVotes        []int64     `json:"my_votes"`
	Options        []OptionRes `json:"options"`
}

// OptionRes 选项与票数。
type OptionRes struct {
	ID      int64  `json:"id"`
	Content string `json:"content"`
	Votes   int64  `json:"votes"`
	Percent int    `json:"percent"`
}

// Service 投票业务。
type Service struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// New 构造。
func New(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{pool: pool, logger: logger}
}

func invalid(msg string) error { return apperr.BadRequest("VALIDATION_ERROR", msg) }

// ValidateConfig 校验投票配置（发帖时调用）。
func ValidateConfig(cfg *Config) error {
	if cfg == nil || len(cfg.Options) < 2 || len(cfg.Options) > 10 {
		return invalid("投票选项需为 2-10 个")
	}
	seen := map[string]bool{}
	for _, o := range cfg.Options {
		o = trim(o)
		if o == "" || len([]rune(o)) > 200 {
			return invalid("选项内容需为 1-200 字")
		}
		if seen[o] {
			return invalid("选项内容重复")
		}
		seen[o] = true
	}
	if cfg.MultipleChoice {
		if cfg.MaxChoices < 2 {
			cfg.MaxChoices = len(cfg.Options)
		}
		if cfg.MaxChoices > len(cfg.Options) {
			return invalid("最多可选数不能超过选项数")
		}
	} else {
		cfg.MaxChoices = 1
	}
	if cfg.EndsAt != nil && cfg.EndsAt.Before(time.Now().UTC()) {
		return invalid("投票截止时间必须晚于当前时间")
	}
	return nil
}

// CreateForPostTx 在帖子事务内创建投票（post_type=poll 时调用）。
func (s *Service) CreateForPostTx(ctx context.Context, tx pgx.Tx, postID int64, cfg *Config) error {
	if err := ValidateConfig(cfg); err != nil {
		return err
	}
	var pollID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO polls (post_id, multiple_choice, max_choices, allow_revote, ends_at)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		postID, cfg.MultipleChoice, cfg.MaxChoices, cfg.AllowRevote, cfg.EndsAt).Scan(&pollID); err != nil {
		return err
	}
	for i, content := range cfg.Options {
		if _, err := tx.Exec(ctx,
			`INSERT INTO poll_options (poll_id, content, sort_order) VALUES ($1, $2, $3)`,
			pollID, trim(content), i); err != nil {
			return err
		}
	}
	return nil
}

// getByPost 取投票定义。
func (s *Service) getByPost(ctx context.Context, postID int64) (*Poll, error) {
	var p Poll
	err := s.pool.QueryRow(ctx, `
		SELECT id, post_id, multiple_choice, max_choices, allow_revote, ends_at
		FROM polls WHERE post_id = $1`, postID).
		Scan(&p.ID, &p.PostID, &p.MultipleChoice, &p.MaxChoices, &p.AllowRevote, &p.EndsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("该帖子没有投票")
	}
	if err != nil {
		return nil, err
	}
	p.Expired = p.EndsAt != nil && p.EndsAt.Before(time.Now().UTC())
	return &p, nil
}

// Results 投票结果（含我的投票）。
func (s *Service) Results(ctx context.Context, postPublicID string, viewerID int64) (*Poll, error) {
	var postID int64
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM posts WHERE public_id = $1 AND deleted_at IS NULL`, postPublicID).Scan(&postID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("帖子不存在")
	}
	if err != nil {
		return nil, err
	}
	p, err := s.getByPost(ctx, postID)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT o.id, o.content,
		       (SELECT COUNT(*) FROM poll_votes v WHERE v.option_id = o.id)
		FROM poll_options o WHERE o.poll_id = $1 ORDER BY o.sort_order`, p.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	p.Options = []OptionRes{}
	var totalVotes int64
	for rows.Next() {
		var o OptionRes
		if err := rows.Scan(&o.ID, &o.Content, &o.Votes); err != nil {
			return nil, err
		}
		totalVotes += o.Votes
		p.Options = append(p.Options, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range p.Options {
		if totalVotes > 0 {
			p.Options[i].Percent = int(float64(p.Options[i].Votes) / float64(totalVotes) * 100)
		}
	}
	_ = s.pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT user_id) FROM poll_votes WHERE poll_id = $1`, p.ID).Scan(&p.TotalVoters)
	if viewerID > 0 {
		vrows, err := s.pool.Query(ctx,
			`SELECT option_id FROM poll_votes WHERE poll_id = $1 AND user_id = $2`, p.ID, viewerID)
		if err == nil {
			defer vrows.Close()
			for vrows.Next() {
				var oid int64
				if err := vrows.Scan(&oid); err == nil {
					p.MyVotes = append(p.MyVotes, oid)
				}
			}
		}
	}
	if p.MyVotes == nil {
		p.MyVotes = []int64{}
	}
	return p, nil
}

// Vote 投票。单选一次一票；多选 1..max_choices；allow_revote 时可改票。
// 并发安全：FOR UPDATE 锁定投票行 + 数据库唯一约束兜底。
func (s *Service) Vote(ctx context.Context, userID int64, postPublicID string, optionIDs []int64) (*Poll, error) {
	var postID int64
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM posts WHERE public_id = $1 AND deleted_at IS NULL AND status = 'published'`, postPublicID).Scan(&postID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("帖子不存在")
	}
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer tx.Rollback(ctx)

	// 行锁防并发改票/防超限
	var p Poll
	err = tx.QueryRow(ctx, `
		SELECT id, multiple_choice, max_choices, allow_revote, ends_at
		FROM polls WHERE post_id = $1 FOR UPDATE`, postID).
		Scan(&p.ID, &p.MultipleChoice, &p.MaxChoices, &p.AllowRevote, &p.EndsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("该帖子没有投票")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if p.EndsAt != nil && p.EndsAt.Before(time.Now().UTC()) {
		return nil, apperr.Conflict("CONFLICT", "投票已截止")
	}
	// 去重入参
	seen := map[int64]bool{}
	unique := make([]int64, 0, len(optionIDs))
	for _, id := range optionIDs {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return nil, invalid("请选择至少一个选项")
	}
	if p.MultipleChoice {
		if len(unique) > p.MaxChoices {
			return nil, invalid("最多可选 " + itoa(p.MaxChoices) + " 项")
		}
	} else if len(unique) > 1 {
		return nil, invalid("该投票为单选")
	}
	// 校验选项归属
	for _, oid := range unique {
		var belongs bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM poll_options WHERE id = $1 AND poll_id = $2)`, oid, p.ID).Scan(&belongs); err != nil {
			return nil, apperr.Internal(err)
		}
		if !belongs {
			return nil, invalid("选项不属于该投票")
		}
	}
	// 已有投票处理
	var existing int
	_ = tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM poll_votes WHERE poll_id = $1 AND user_id = $2`, p.ID, userID).Scan(&existing)
	if existing > 0 && !p.AllowRevote {
		return nil, apperr.Conflict("CONFLICT", "你已经投过票了")
	}
	if existing > 0 {
		if _, err := tx.Exec(ctx,
			`DELETE FROM poll_votes WHERE poll_id = $1 AND user_id = $2`, p.ID, userID); err != nil {
			return nil, apperr.Internal(err)
		}
	}
	for _, oid := range unique {
		if _, err := tx.Exec(ctx,
			`INSERT INTO poll_votes (poll_id, option_id, user_id) VALUES ($1, $2, $3)
			 ON CONFLICT (user_id, option_id) DO NOTHING`, p.ID, oid, userID); err != nil {
			return nil, apperr.Internal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, apperr.Internal(err)
	}
	s.logger.Info("poll voted", "post_id", postID, "user_id", userID)
	return s.Results(ctx, postPublicID, userID)
}

func trim(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// ---- HTTP ----

// Handler 投票 HTTP 层。
type Handler struct{ svc *Service }

// NewHandler 构造。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register 注册路由。
func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/posts/:id/poll", h.Results)
	vote := rg.Group("")
	vote.Use(httpx.RequireAuth(), httpx.RequireVerified())
	vote.POST("/posts/:id/poll/vote", h.Vote)
}

// Results GET /posts/:id/poll
func (h *Handler) Results(c *gin.Context) {
	var viewerID int64
	if u := httpx.CurrentUser(c); u != nil {
		viewerID = u.ID
	}
	p, err := h.svc.Results(c.Request.Context(), c.Param("id"), viewerID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"poll": p})
}

// Vote POST /posts/:id/poll/vote
func (h *Handler) Vote(c *gin.Context) {
	u := httpx.CurrentUser(c)
	var req struct {
		OptionIDs []int64 `json:"option_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, invalid("请求体格式不正确"))
		return
	}
	p, err := h.svc.Vote(c.Request.Context(), u.ID, c.Param("id"), req.OptionIDs)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"poll": p, "message": "投票成功"})
}

// Package outbox 实现事务性发件箱：业务事务内写事件，提交后由 Dispatcher 异步投递。
// 保证「主事务失败则事件不存在；事件投递失败可重试」，禁止先发通知后提交主事务。
package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler 单个事件类型的投递处理器。
type Handler func(ctx context.Context, aggregateID int64, payload json.RawMessage) error

// EmitTx 在业务事务内写入待投递事件。
func EmitTx(ctx context.Context, tx pgx.Tx, eventType string, aggregateID int64, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化 outbox 事件失败: %w", err)
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO outbox_events (event_type, aggregate_id, payload) VALUES ($1, $2, $3)`,
		eventType, aggregateID, b)
	return err
}

// Dispatcher 轮询 outbox 并投递事件（API 进程内嵌 + 独立 worker 同构使用）。
type Dispatcher struct {
	pool     *pgxpool.Pool
	logger   *slog.Logger
	handlers map[string]Handler

	interval     time.Duration
	batchSize    int
	maxAttempts  int
	claimTimeout time.Duration
}

// NewDispatcher 构造投递器。
func NewDispatcher(pool *pgxpool.Pool, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{
		pool:         pool,
		logger:       logger,
		handlers:     map[string]Handler{},
		interval:     time.Second,
		batchSize:    20,
		maxAttempts:  16,
		claimTimeout: 5 * time.Minute,
	}
}

// Register 注册事件处理器（重复注册覆盖）。
func (d *Dispatcher) Register(eventType string, h Handler) { d.handlers[eventType] = h }

// Run 阻塞式投递循环；ctx 取消即退出。
func (d *Dispatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.processOnce(ctx)
		}
	}
}

type event struct {
	id          int64
	eventType   string
	aggregateID int64
	payload     []byte
	attempts    int
}

func (d *Dispatcher) processOnce(ctx context.Context) {
	// 归还卡死事件
	_, _ = d.pool.Exec(ctx, `
		UPDATE outbox_events SET status = 'pending', next_retry_at = now_utc()
		WHERE status = 'processing' AND next_retry_at < now_utc() - ($1::text)::interval`,
		"5 minutes")

	rows, err := d.pool.Query(ctx, `
		UPDATE outbox_events SET status = 'processing', next_retry_at = now_utc()
		WHERE id IN (
			SELECT id FROM outbox_events
			WHERE status = 'pending' AND next_retry_at <= now_utc()
			ORDER BY id
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, event_type, aggregate_id, payload, attempts`, d.batchSize)
	if err != nil {
		d.logger.Error("outbox 认领失败", "err", err)
		return
	}
	var events []event
	for rows.Next() {
		var e event
		if err := rows.Scan(&e.id, &e.eventType, &e.aggregateID, &e.payload, &e.attempts); err == nil {
			events = append(events, e)
		}
	}
	rows.Close()
	if len(events) == 0 {
		return
	}

	for _, e := range events {
		h, ok := d.handlers[e.eventType]
		if !ok {
			d.finish(ctx, e, fmt.Errorf("未注册的事件类型 %s", e.eventType))
			continue
		}
		hctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := h(hctx, e.aggregateID, e.payload)
		cancel()
		d.finish(ctx, e, err)
	}
}

func (d *Dispatcher) finish(ctx context.Context, e event, err error) {
	if err == nil {
		_, _ = d.pool.Exec(ctx,
			`UPDATE outbox_events SET status='done', processed_at=now_utc(), last_error='' WHERE id=$1`, e.id)
		return
	}
	next := e.attempts + 1
	if next >= d.maxAttempts {
		_, _ = d.pool.Exec(ctx,
			`UPDATE outbox_events SET status='failed', attempts=$2, last_error=$3 WHERE id=$1`, e.id, next, err.Error())
		d.logger.Error("outbox 事件最终失败", "event_id", e.id, "type", e.eventType, "err", err)
		return
	}
	// 指数退避：2^n 秒，上限 10 分钟
	backoff := time.Duration(1<<uint(min(next, 10))) * time.Second
	_, _ = d.pool.Exec(ctx,
		`UPDATE outbox_events SET status='pending', attempts=$2, last_error=$3,
		   next_retry_at = now_utc() + ($4::text)::interval
		 WHERE id=$1`, e.id, next, err.Error(), formatInterval(backoff))
	d.logger.Warn("outbox 事件投递失败，稍后重试", "event_id", e.id, "attempt", next, "err", err)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatInterval(d time.Duration) string {
	secs := int(d.Seconds())
	return fmt.Sprintf("%d seconds", secs)
}

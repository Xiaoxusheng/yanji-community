package notifications

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/yanji-community/backend/internal/platform/config"
	"github.com/yanji-community/backend/internal/platform/mailer"
	"github.com/yanji-community/backend/internal/shared/outbox"
)

// RegisterEmailHandlers 注册邮件类 Outbox 消费者（API 与 worker 进程共用；
// outbox 采用 FOR UPDATE SKIP LOCKED 认领，多进程并发不重复投递）。
func RegisterEmailHandlers(d *outbox.Dispatcher, m mailer.Mailer, cfg *config.Config) {
	send := func(ctx context.Context, to, subject, body string) error {
		if to == "" {
			return errors.New("邮件事件缺少收件人")
		}
		return m.Send(ctx, to, subject, body)
	}
	link := func(path, token string) string { return cfg.PublicURL + path + token }

	d.Register("email.verification", func(ctx context.Context, _ int64, raw json.RawMessage) error {
		var p struct {
			To, Username, Token string
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		body := "你好 " + p.Username + "：\n\n请点击以下链接完成邮箱验证（24 小时内有效）：" +
			link("/verify-email?token=", p.Token) + "\n\n如非本人操作请忽略本邮件。"
		return send(ctx, p.To, "【研记社区】邮箱验证", body)
	})

	d.Register("email.password_reset", func(ctx context.Context, _ int64, raw json.RawMessage) error {
		var p struct {
			To, Username, Token string
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		body := "你好 " + p.Username + "：\n\n请点击以下链接重置密码（30 分钟内有效）：" +
			link("/reset-password?token=", p.Token) + "\n\n如非本人操作请忽略本邮件并检查账号安全。"
		return send(ctx, p.To, "【研记社区】密码重置", body)
	})

	d.Register("email.welcome", func(ctx context.Context, _ int64, raw json.RawMessage) error {
		var p struct {
			To, Username string
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		body := "你好 " + p.Username + "：\n\n邮箱验证成功，欢迎加入研记社区！现在可以去发布你的第一篇帖子了。"
		return send(ctx, p.To, "【研记社区】欢迎加入", body)
	})
}

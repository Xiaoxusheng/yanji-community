// Package mailer 提供邮件发送抽象：开发环境写日志，生产可切换 SMTP。
// 通过 Outbox 事件异步触发，本包不做业务判断。
package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
)

// Mailer 邮件发送接口。
type Mailer interface {
	Send(ctx context.Context, to, subject, textBody string) error
}

// ---- 日志实现（开发环境默认）----

// LogMailer 将邮件内容写日志，便于本地联调验证链接。
type LogMailer struct {
	logger *slog.Logger
}

// NewLogMailer 构造日志邮件发送器。
func NewLogMailer(l *slog.Logger) *LogMailer { return &LogMailer{logger: l} }

// Send 记录邮件到日志。
func (m *LogMailer) Send(_ context.Context, to, subject, body string) error {
	m.logger.Info("MAIL(log)", "to", to, "subject", subject, "body", body)
	return nil
}

// ---- SMTP 实现 ----

// SMTPMailer 通过 SMTP 发送邮件。
type SMTPMailer struct {
	host     string
	port     int
	username string
	password string
	from     string
	logger   *slog.Logger
}

// NewSMTPMailer 构造 SMTP 发送器。
func NewSMTPMailer(host string, port int, username, password, from string, l *slog.Logger) *SMTPMailer {
	return &SMTPMailer{host: host, port: port, username: username, password: password, from: from, logger: l}
}

// Send 发送纯文本邮件。
func (m *SMTPMailer) Send(_ context.Context, to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", m.host, m.port)
	msg := strings.Join([]string{
		"From: " + m.from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	auth := smtp.PlainAuth("", m.username, m.password, m.host)
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: m.host})
	if err != nil {
		return fmt.Errorf("SMTP 连接失败: %w", err)
	}
	defer conn.Close()
	cli, err := smtp.NewClient(conn, m.host)
	if err != nil {
		return err
	}
	defer cli.Close()
	if ok, _ := cli.Extension("AUTH"); ok {
		if err := cli.Auth(auth); err != nil {
			return fmt.Errorf("SMTP 认证失败: %w", err)
		}
	}
	if err := cli.Mail(m.from); err != nil {
		return err
	}
	if err := cli.Rcpt(to); err != nil {
		return err
	}
	w, err := cli.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	m.logger.Info("MAIL(smtp)", "to", to, "subject", subject)
	return cli.Quit()
}

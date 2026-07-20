// Package email is the outbound-email adapter. A real SMTP sender is used when
// SMTP_HOST is configured; otherwise a mock that logs the message (so flows like
// password reset are fully testable in dev without a mail server). Same pattern
// as the AI/MinIO adapters — swapping is config-driven.
package email

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"

	"github.com/tararahuuw/ragsytem/internal/config"
)

// Sender delivers a plain-text email.
type Sender interface {
	Send(ctx context.Context, to, subject, body string) error
}

// NewSender picks SMTP when configured, otherwise the logging mock.
func NewSender(cfg *config.Config) Sender {
	if cfg.SMTPHost == "" {
		slog.Warn("email: SMTP_HOST not set — using MOCK email sender (emails are logged, not sent)")
		return &logSender{}
	}
	return &smtpSender{
		addr:     net.JoinHostPort(cfg.SMTPHost, cfg.SMTPPort),
		host:     cfg.SMTPHost,
		username: cfg.SMTPUsername,
		password: cfg.SMTPPassword,
		from:     cfg.SMTPFrom,
	}
}

// logSender logs the email instead of sending it (dev only).
type logSender struct{}

func (l *logSender) Send(_ context.Context, to, subject, body string) error {
	slog.Warn("email (MOCK, not sent)", "to", to, "subject", subject, "body", body)
	return nil
}

type smtpSender struct {
	addr, host, username, password, from string
}

func (s *smtpSender) Send(_ context.Context, to, subject, body string) error {
	var auth smtp.Auth
	if s.username != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}
	msg := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		s.from, to, subject, body,
	))
	if err := smtp.SendMail(s.addr, auth, s.from, []string{to}, msg); err != nil {
		return fmt.Errorf("email: send to %s: %w", to, err)
	}
	return nil
}

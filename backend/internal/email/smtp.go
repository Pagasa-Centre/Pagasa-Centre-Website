package email

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
)

// SMTPMailer sends real email via SMTP (Gmail / Google Workspace with an app
// password is the expected production setup).
type SMTPMailer struct {
	host     string
	port     string
	username string
	password string
	from     string
}

// NewSMTPMailer constructs an SMTPMailer. All fields are required.
func NewSMTPMailer(host, port, username, password, from string) *SMTPMailer {
	return &SMTPMailer{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

// SendDepositConfirmation renders the deposit/day-pass confirmation template
// and dispatches it over SMTP.
func (s *SMTPMailer) SendDepositConfirmation(ctx context.Context, p DepositConfirmation) error {
	subject, body, err := renderDepositConfirmation(p)
	if err != nil {
		return err
	}
	return s.send(ctx, p.ToEmail, subject, body)
}

func (s *SMTPMailer) send(_ context.Context, to, subject, htmlBody string) error {
	if to == "" {
		return fmt.Errorf("empty recipient")
	}
	auth := smtp.PlainAuth("", s.username, s.password, s.host)
	addr := s.host + ":" + s.port

	headers := []string{
		"From: " + s.from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		`Content-Type: text/html; charset="utf-8"`,
	}
	msg := strings.Join(headers, "\r\n") + "\r\n\r\n" + htmlBody

	if err := smtp.SendMail(addr, auth, s.from, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("smtp send to %s: %w", to, err)
	}
	return nil
}

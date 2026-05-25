package email

import (
	"context"
	"fmt"
	"net/mail"
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

	// `From` env var may be either a bare address ("foo@bar.com") or RFC 5322
	// display-name form ("Name <foo@bar.com>"). The header keeps the full
	// thing; the SMTP envelope-MAIL-FROM only accepts the bare address, and
	// Gmail rejects display-name syntax there with 501 5.1.7.
	envelopeFrom := envelopeAddress(s.from, s.username)

	headers := []string{
		"From: " + s.from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		`Content-Type: text/html; charset="utf-8"`,
	}
	msg := strings.Join(headers, "\r\n") + "\r\n\r\n" + htmlBody

	if err := smtp.SendMail(addr, auth, envelopeFrom, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("smtp send to %s: %w", to, err)
	}
	return nil
}

// envelopeAddress extracts the bare email from an RFC 5322 address string.
// Falls back to `fallback` if parsing fails so we never end up with an empty
// envelope-from.
func envelopeAddress(raw, fallback string) string {
	if a, err := mail.ParseAddress(raw); err == nil && a.Address != "" {
		return a.Address
	}
	if a, err := mail.ParseAddress(fallback); err == nil && a.Address != "" {
		return a.Address
	}
	return fallback
}

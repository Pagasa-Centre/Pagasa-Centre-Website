package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
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

func (s *SMTPMailer) SendAllocationReleased(ctx context.Context, p AllocationReleased) error {
	subject, body, err := renderAllocationReleased(p)
	if err != nil {
		return err
	}
	return s.send(ctx, p.ToEmail, subject, body)
}

func (s *SMTPMailer) SendWhiteTeamNotification(ctx context.Context, p WhiteTeamNotification) error {
	subject, body, err := renderWhiteTeamNotification(p)
	if err != nil {
		return err
	}
	return s.send(ctx, p.ToEmail, subject, body)
}

func (s *SMTPMailer) SendBalanceInvoice(ctx context.Context, p BalanceInvoice) error {
	subject, body, err := renderBalanceInvoice(p)
	if err != nil {
		return err
	}
	return s.send(ctx, p.ToEmail, subject, body)
}

func (s *SMTPMailer) SendCoachInvoice(ctx context.Context, p CoachInvoice) error {
	subject, body, err := renderCoachInvoice(p)
	if err != nil {
		return err
	}
	return s.send(ctx, p.ToEmail, subject, body)
}

func (s *SMTPMailer) SendBalancePaid(ctx context.Context, p BalancePaid) error {
	subject, body, err := renderBalancePaid(p)
	if err != nil {
		return err
	}
	return s.send(ctx, p.ToEmail, subject, body)
}

func (s *SMTPMailer) SendBalancePaidConfirmation(ctx context.Context, p BalancePaidConfirmation) error {
	subject, body, err := renderBalancePaidConfirmation(p)
	if err != nil {
		return err
	}
	return s.send(ctx, p.ToEmail, subject, body)
}

func (s *SMTPMailer) SendSponsorshipConfirmed(ctx context.Context, p SponsorshipConfirmed) error {
	subject, body, err := renderSponsorshipConfirmed(p)
	if err != nil {
		return err
	}
	return s.send(ctx, p.ToEmail, subject, body)
}

func (s *SMTPMailer) SendAccommodationChanged(ctx context.Context, p AccommodationChangedNotice) error {
	subject, body, err := renderAccommodationChanged(p)
	if err != nil {
		return err
	}
	return s.send(ctx, p.ToEmail, subject, body)
}

func (s *SMTPMailer) SendBookingUpdated(ctx context.Context, p BookingUpdated) error {
	subject, body, err := renderBookingUpdated(p)
	if err != nil {
		return err
	}
	return s.send(ctx, p.ToEmail, subject, body)
}

func (s *SMTPMailer) send(_ context.Context, to, subject, htmlBody string) error {
	if to == "" {
		return fmt.Errorf("empty recipient")
	}

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
	msg := []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + htmlBody)

	// Port 465 = implicit TLS from connection start (SMTPS). Required on
	// hosts like Railway/Fly that block 587. Port 587 = STARTTLS upgrade
	// from plaintext, which is the original net/smtp.SendMail behaviour.
	if s.port == "465" {
		return s.sendImplicitTLS(envelopeFrom, to, msg)
	}
	return s.sendSTARTTLS(envelopeFrom, to, msg)
}

func (s *SMTPMailer) sendSTARTTLS(from, to string, msg []byte) error {
	auth := smtp.PlainAuth("", s.username, s.password, s.host)
	addr := s.host + ":" + s.port
	if err := smtp.SendMail(addr, auth, from, []string{to}, msg); err != nil {
		return fmt.Errorf("smtp send to %s: %w", to, err)
	}
	return nil
}

func (s *SMTPMailer) sendImplicitTLS(from, to string, msg []byte) error {
	addr := s.host + ":" + s.port
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	tlsConn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: s.host})
	if err != nil {
		return fmt.Errorf("smtp dial %s: %w", addr, err)
	}
	defer tlsConn.Close()

	client, err := smtp.NewClient(tlsConn, s.host)
	if err != nil {
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer client.Quit()

	auth := smtp.PlainAuth("", s.username, s.password, s.host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp RCPT TO %s: %w", to, err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close body: %w", err)
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

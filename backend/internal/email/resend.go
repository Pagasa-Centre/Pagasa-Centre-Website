package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ResendMailer sends transactional email via the Resend HTTP API
// (https://resend.com). Used when the hosting platform blocks outbound SMTP
// ports (Railway, Fly, Render etc. all do).
//
// Resend's free tier: 3,000/month + 100/day, sufficient for camp registration
// volumes. The free tier requires using their verified `onboarding@resend.dev`
// from-address OR adding DNS records to verify your own domain.
type ResendMailer struct {
	apiKey string
	from   string
	client *http.Client
}

// NewResendMailer constructs a ResendMailer. apiKey is the `re_...` value from
// the Resend dashboard, from is the verified sender address.
func NewResendMailer(apiKey, from string) *ResendMailer {
	return &ResendMailer{
		apiKey: apiKey,
		from:   from,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// SendDepositConfirmation renders the deposit/day-pass confirmation template
// and dispatches it via the Resend HTTP API.
func (r *ResendMailer) SendDepositConfirmation(ctx context.Context, p DepositConfirmation) error {
	subject, body, err := renderDepositConfirmation(p)
	if err != nil {
		return err
	}
	return r.send(ctx, p.ToEmail, subject, body)
}

func (r *ResendMailer) SendAllocationReleased(ctx context.Context, p AllocationReleased) error {
	subject, body, err := renderAllocationReleased(p)
	if err != nil {
		return err
	}
	return r.send(ctx, p.ToEmail, subject, body)
}

func (r *ResendMailer) SendWhiteTeamNotification(ctx context.Context, p WhiteTeamNotification) error {
	subject, body, err := renderWhiteTeamNotification(p)
	if err != nil {
		return err
	}
	return r.send(ctx, p.ToEmail, subject, body)
}

func (r *ResendMailer) SendBalanceInvoice(ctx context.Context, p BalanceInvoice) error {
	subject, body, err := renderBalanceInvoice(p)
	if err != nil {
		return err
	}
	return r.send(ctx, p.ToEmail, subject, body)
}

func (r *ResendMailer) SendBalancePaid(ctx context.Context, p BalancePaid) error {
	subject, body, err := renderBalancePaid(p)
	if err != nil {
		return err
	}
	return r.send(ctx, p.ToEmail, subject, body)
}

func (r *ResendMailer) SendBalancePaidConfirmation(ctx context.Context, p BalancePaidConfirmation) error {
	subject, body, err := renderBalancePaidConfirmation(p)
	if err != nil {
		return err
	}
	return r.send(ctx, p.ToEmail, subject, body)
}

func (r *ResendMailer) SendSponsorshipConfirmed(ctx context.Context, p SponsorshipConfirmed) error {
	subject, body, err := renderSponsorshipConfirmed(p)
	if err != nil {
		return err
	}
	return r.send(ctx, p.ToEmail, subject, body)
}

func (r *ResendMailer) SendAccommodationChanged(ctx context.Context, p AccommodationChangedNotice) error {
	subject, body, err := renderAccommodationChanged(p)
	if err != nil {
		return err
	}
	return r.send(ctx, p.ToEmail, subject, body)
}

type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

type resendErrorBody struct {
	Message    string `json:"message"`
	Name       string `json:"name"`
	StatusCode int    `json:"statusCode"`
}

func (r *ResendMailer) send(ctx context.Context, to, subject, htmlBody string) error {
	if to == "" {
		return fmt.Errorf("empty recipient")
	}
	payload, err := json.Marshal(resendRequest{
		From:    r.from,
		To:      []string{to},
		Subject: subject,
		HTML:    htmlBody,
	})
	if err != nil {
		return fmt.Errorf("marshal resend request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("resend POST: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var apiErr resendErrorBody
	if json.Unmarshal(bodyBytes, &apiErr) == nil && apiErr.Message != "" {
		return fmt.Errorf("resend %d %s: %s", resp.StatusCode, apiErr.Name, apiErr.Message)
	}
	return fmt.Errorf("resend %d: %s", resp.StatusCode, string(bodyBytes))
}

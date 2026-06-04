package email

import (
	"bytes"
	"fmt"
	"html/template"
)

// Key dates from the leaders' meeting (24 May 2026). Hardcoded for v1 — admins
// don't need to edit these mid-campaign. If we run another camp in a future
// year these should move to the camp_config table.
const (
	KeyDatesRegistration  = "1 - 30 June"
	KeyDatesAllocation    = "1 - 15 July"
	KeyDatesFinalPayment  = "26 - 31 July"
	AllocationContactName = "Bro Ash"
)

type templateData struct {
	ToName               string
	AmountFormatted      string // "£100.00" when AmountPence > 0, empty otherwise
	CamperCount          int
	HasMinor             bool
	ConsentFormURL       string
	IsDepositConfirm     bool // true when AmountPence > 0
	KeyDatesRegistration string
	KeyDatesAllocation   string
	KeyDatesFinalPayment string
	BroAsh               string
}

// renderDepositConfirmation builds the (subject, htmlBody) for a deposit /
// day-pass confirmation email.
func renderDepositConfirmation(p DepositConfirmation) (subject, htmlBody string, err error) {
	td := templateData{
		ToName:               p.ToName,
		CamperCount:          p.CamperCount,
		HasMinor:             p.HasMinor,
		ConsentFormURL:       p.ConsentFormURL,
		IsDepositConfirm:     p.AmountPence > 0,
		KeyDatesRegistration: KeyDatesRegistration,
		KeyDatesAllocation:   KeyDatesAllocation,
		KeyDatesFinalPayment: KeyDatesFinalPayment,
		BroAsh:               AllocationContactName,
	}
	if td.IsDepositConfirm {
		td.AmountFormatted = formatPence(p.AmountPence, p.Currency)
		subject = "Your PC Summer Camp 2026 deposit has been received"
	} else {
		subject = "Your PC Summer Camp 2026 registration is confirmed"
	}

	tmpl, err := template.New("deposit").Parse(depositConfirmationHTML)
	if err != nil {
		return "", "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, td); err != nil {
		return "", "", fmt.Errorf("execute template: %w", err)
	}
	return subject, buf.String(), nil
}

// formatPence renders an integer pence amount as e.g. "£100.00".
func formatPence(pence int, currency string) string {
	if currency == "" {
		currency = "GBP"
	}
	symbol := "£"
	if currency != "GBP" {
		symbol = currency + " "
	}
	return fmt.Sprintf("%s%d.%02d", symbol, pence/100, pence%100)
}

func renderAllocationReleased(p AllocationReleased) (subject, htmlBody string, err error) {
	subject = "PC Summer Camp 2026 — accommodation allocation released"
	names := ""
	for _, n := range p.CamperNames {
		names += "<li>" + template.HTMLEscapeString(n) + "</li>"
	}
	reason := template.HTMLEscapeString(p.Reason)
	htmlBody = fmt.Sprintf(`<!DOCTYPE html><html><body style="font-family:sans-serif;color:#333">
<p>Dear %s,</p>
<p>Your temporary accommodation allocation for PC Summer Camp 2026 has been released because the balance invoice was not paid by the due date.</p>
<p><strong>Reason:</strong> %s</p>
<ul>%s</ul>
<p>If you still wish to attend, please speak to your cell leader or the White Team. You may register again if places are still available.</p>
<p>God bless,<br>Pag-Asa Centre</p>
</body></html>`, template.HTMLEscapeString(p.ToName), reason, names)
	return subject, htmlBody, nil
}

func renderWhiteTeamNotification(p WhiteTeamNotification) (subject, htmlBody string, err error) {
	subject = p.Subject
	body := template.HTMLEscapeString(p.Body)
	htmlBody = fmt.Sprintf(`<!DOCTYPE html><html><body style="font-family:sans-serif;white-space:pre-wrap">%s</body></html>`, body)
	return subject, htmlBody, nil
}

const depositConfirmationHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>PC Summer Camp 2026</title>
</head>
<body style="font-family: -apple-system, Segoe UI, Helvetica, Arial, sans-serif; color: #1a1a1a; line-height: 1.55; max-width: 600px; margin: 0 auto; padding: 24px;">
  <h1 style="font-size: 22px; margin: 0 0 16px;">{{if .IsDepositConfirm}}Your deposit has been received{{else}}Your registration is confirmed{{end}}</h1>

  <p>Hi {{if .ToName}}{{.ToName}}{{else}}there{{end}},</p>

  {{if .IsDepositConfirm}}
  <p>Thank you for registering for <strong>PC Summer Camp 2026</strong>. We've received your non-refundable deposit of <strong>{{.AmountFormatted}}</strong> covering {{.CamperCount}} camper{{if ne .CamperCount 1}}s{{end}}. A separate payment receipt has been emailed to you by Stripe.</p>
  {{else}}
  <p>Thank you for registering {{.CamperCount}} camper{{if ne .CamperCount 1}}s{{end}} for <strong>PC Summer Camp 2026</strong>. Day-pass attendance doesn't require a deposit — any catering or t-shirt fees will be settled directly with the camp team.</p>
  {{end}}

  <h2 style="font-size: 16px; margin: 24px 0 8px;">What happens next</h2>
  <ol style="padding-left: 20px;">
    <li><strong>{{.KeyDatesRegistration}}</strong> — Registration is open.</li>
    <li><strong>{{.KeyDatesAllocation}}</strong> — The committee allocates rooms. You'll hear from your cell leader, by email, in person, or at arrival on the day.</li>
    <li><strong>{{.KeyDatesFinalPayment}}</strong> — Final payment window. Once the balance is settled, your accommodation is fully confirmed.</li>
  </ol>

  {{if .HasMinor}}
  <h2 style="font-size: 16px; margin: 24px 0 8px;">Parental consent form</h2>
  <p>One or more of the campers on your registration is under 18. Please <strong>print and sign</strong> the parental consent form and hand it to {{.BroAsh}} before the start of camp. We don't accept email scans.</p>
  {{if .ConsentFormURL}}
  <p><a href="{{.ConsentFormURL}}" style="display: inline-block; background: #006848; color: #fff; padding: 10px 18px; text-decoration: none; font-weight: bold;">Download Parental Consent Form</a></p>
  {{end}}
  {{end}}

  <h2 style="font-size: 16px; margin: 24px 0 8px;">Questions?</h2>
  <p>If anything looks wrong, reply to this email or speak to your cell leader.</p>

  <p style="margin-top: 32px;">God bless,<br>The PC Summer Camp 2026 team</p>
</body>
</html>`

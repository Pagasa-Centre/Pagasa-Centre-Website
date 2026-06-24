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
	KeyDatesRegistration  = "15 - 30 June"
	KeyDatesAllocation    = "1 - 15 July"
	KeyDatesFinalPayment  = "16 - 31 July"
	AllocationContactName = "Bro Ash"
)

type templateData struct {
	ToName               string
	AmountFormatted      string // "£100.00" when AmountPence > 0, empty otherwise
	CamperCount          int
	HasMinor             bool
	ConsentFormURL       string
	IsDepositConfirm     bool // true when AmountPence > 0
	IsFree               bool // church-sponsored registration
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
		IsFree:               p.IsFree,
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

func renderBalanceInvoice(p BalanceInvoice) (subject, htmlBody string, err error) {
	subject = "Your PC Summer Camp 2026 balance is ready to pay"
	names := ""
	for _, n := range p.Items {
		names += "<li>" + template.HTMLEscapeString(n) + "</li>"
	}
	due := ""
	if p.DueDate != "" {
		due = fmt.Sprintf(
			`<p>Please pay by <strong>%s</strong>. If the balance is unpaid by then, the accommodation may be released.</p>`,
			template.HTMLEscapeString(p.DueDate))
	}
	amount := ""
	if p.AmountLabel != "" {
		amount = fmt.Sprintf(`<p>Amount due: <strong>%s</strong></p>`, template.HTMLEscapeString(p.AmountLabel))
	}
	camperBlock := ""
	if names != "" {
		camperBlock = "<p>This payment covers:</p><ul>" + names + "</ul>"
	}
	htmlBody = fmt.Sprintf(`<!DOCTYPE html>
<html lang="en"><body style="font-family: -apple-system, Segoe UI, Helvetica, Arial, sans-serif; color:#1a1a1a; line-height:1.55; max-width:600px; margin:0 auto; padding:24px;">
  <h1 style="font-size:22px; margin:0 0 16px;">Your camp balance is ready to pay</h1>
  <p>Hi %s,</p>
  <p>Your accommodation for <strong>PC Summer Camp 2026</strong> has been allocated. Please pay your remaining balance to confirm your place.</p>
  %s
  %s
  <p style="margin:24px 0;">
    <a href="%s" style="display:inline-block; background:#3ea463; color:#fff; padding:12px 22px; text-decoration:none; font-weight:bold; border-radius:8px;">Pay your balance</a>
  </p>
  <p style="font-size:13px; color:#666;">Or copy this link into your browser:<br>%s</p>
  %s
  <p style="margin-top:32px;">God bless,<br>The PC Summer Camp 2026 team</p>
</body></html>`,
		template.HTMLEscapeString(p.ToName),
		amount,
		due,
		template.HTMLEscapeString(p.PayURL),
		template.HTMLEscapeString(p.PayURL),
		camperBlock,
	)
	return subject, htmlBody, nil
}

func renderBalancePaid(p BalancePaid) (subject, htmlBody string, err error) {
	name := p.ContactName
	if name == "" {
		name = p.ContactEmail
	}
	subject = "Camp balance paid — " + name
	items := ""
	for _, it := range p.Items {
		items += "<li>" + template.HTMLEscapeString(it) + "</li>"
	}
	itemsBlock := ""
	if items != "" {
		itemsBlock = "<p style=\"margin:0 0 4px;\"><strong>People covered:</strong></p><ul style=\"margin:0 0 16px;\">" + items + "</ul>"
	}
	amount := ""
	if p.AmountLabel != "" {
		amount = fmt.Sprintf(`<tr><td style="padding:2px 12px 2px 0;color:#555;">Amount paid</td><td style="padding:2px 0;font-weight:bold;">%s</td></tr>`,
			template.HTMLEscapeString(p.AmountLabel))
	}
	paidDate := ""
	if p.PaidDate != "" {
		paidDate = fmt.Sprintf(`<tr><td style="padding:2px 12px 2px 0;color:#555;">Paid on</td><td style="padding:2px 0;">%s</td></tr>`,
			template.HTMLEscapeString(p.PaidDate))
	}
	htmlBody = fmt.Sprintf(`<!DOCTYPE html>
<html lang="en"><body style="font-family: -apple-system, Segoe UI, Helvetica, Arial, sans-serif; color:#1a1a1a; line-height:1.55; max-width:600px; margin:0 auto; padding:24px;">
  <h1 style="font-size:20px; margin:0 0 16px;">Balance paid in full ✅</h1>
  <p><strong>%s</strong> has paid their PC Summer Camp 2026 balance.</p>
  <table style="border-collapse:collapse; margin:8px 0 16px; font-size:14px;">
    <tr><td style="padding:2px 12px 2px 0;color:#555;">Paid by</td><td style="padding:2px 0;">%s</td></tr>
    <tr><td style="padding:2px 12px 2px 0;color:#555;">Email</td><td style="padding:2px 0;">%s</td></tr>
    %s
    %s
  </table>
  %s
  <p style="color:#555;font-size:13px;">This is an automatic notification for the White Team.</p>
</body></html>`,
		template.HTMLEscapeString(name),
		template.HTMLEscapeString(name),
		template.HTMLEscapeString(p.ContactEmail),
		amount,
		paidDate,
		itemsBlock,
	)
	return subject, htmlBody, nil
}

func renderBalancePaidConfirmation(p BalancePaidConfirmation) (subject, htmlBody string, err error) {
	subject = "Your PC Summer Camp 2026 place is confirmed"
	name := p.ToName
	if name == "" {
		name = "there"
	}
	items := ""
	for _, it := range p.Items {
		items += "<li>" + template.HTMLEscapeString(it) + "</li>"
	}
	accommodationBlock := ""
	if items != "" {
		accommodationBlock = `<p style="margin:16px 0 4px;"><strong>Your accommodation:</strong></p><ul style="margin:0 0 16px;">` + items + "</ul>"
	}
	amountBlock := ""
	if p.AmountLabel != "" {
		amountBlock = fmt.Sprintf(`<p>We've received your balance payment of <strong>%s</strong>.</p>`,
			template.HTMLEscapeString(p.AmountLabel))
	}
	htmlBody = fmt.Sprintf(`<!DOCTYPE html>
<html lang="en"><body style="font-family: -apple-system, Segoe UI, Helvetica, Arial, sans-serif; color:#1a1a1a; line-height:1.55; max-width:600px; margin:0 auto; padding:24px;">
  <h1 style="font-size:22px; margin:0 0 16px;">Your camp place is confirmed ✅</h1>
  <p>Hi %s,</p>
  %s
  <p>Your balance is paid in full and your accommodation for <strong>PC Summer Camp 2026</strong> is now fully confirmed. There's nothing more to pay.</p>
  %s
  <p>If anything looks wrong, reply to this email or speak to your cell leader.</p>
  <p style="margin-top:32px;">God bless,<br>The PC Summer Camp 2026 team</p>
</body></html>`,
		template.HTMLEscapeString(name),
		amountBlock,
		accommodationBlock,
	)
	return subject, htmlBody, nil
}

func renderSponsorshipConfirmed(p SponsorshipConfirmed) (subject, htmlBody string, err error) {
	subject = "Your PC Summer Camp 2026 place is confirmed"
	name := p.ToName
	if name == "" {
		name = "there"
	}
	items := ""
	for _, it := range p.Items {
		items += "<li>" + template.HTMLEscapeString(it) + "</li>"
	}
	accommodationBlock := ""
	if items != "" {
		accommodationBlock = `<p style="margin:16px 0 4px;"><strong>Your accommodation:</strong></p><ul style="margin:0 0 16px;">` + items + "</ul>"
	}
	htmlBody = fmt.Sprintf(`<!DOCTYPE html>
<html lang="en"><body style="font-family: -apple-system, Segoe UI, Helvetica, Arial, sans-serif; color:#1a1a1a; line-height:1.55; max-width:600px; margin:0 auto; padding:24px;">
  <h1 style="font-size:22px; margin:0 0 16px;">Your camp place is confirmed</h1>
  <p>Hi %s,</p>
  <p>Good news — your accommodation for <strong>PC Summer Camp 2026</strong> has been allocated and your place is fully confirmed. As your registration is sponsored by the church, there is nothing to pay.</p>
  %s
  <p>If anything looks wrong, reply to this email or speak to your cell leader.</p>
  <p style="margin-top:32px;">God bless,<br>The PC Summer Camp 2026 team</p>
</body></html>`,
		template.HTMLEscapeString(name),
		accommodationBlock,
	)
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
  {{else if .IsFree}}
  <p>Thank you for registering {{.CamperCount}} camper{{if ne .CamperCount 1}}s{{end}} for <strong>PC Summer Camp 2026</strong>. Your registration is fully sponsored by the church — there is nothing to pay.</p>
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

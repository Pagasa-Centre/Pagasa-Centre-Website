package email

import (
	"fmt"
	"html/template"
	"strings"
	"time"

	"pagasacentre/backend/internal/notify"
)

func formatDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2 Jan 2006")
}

func amountLabel(a notify.Amount) string {
	if a.Pence <= 0 {
		return ""
	}
	symbol := "£"
	if cur := strings.ToUpper(a.Currency); cur != "GBP" && cur != "" {
		symbol = cur + " "
	}
	return fmt.Sprintf("%s%d.%02d", symbol, a.Pence/100, a.Pence%100)
}

func itemLine(it notify.InvoiceItem) string {
	switch it.Kind {
	case notify.ItemDayPass:
		unit := "day"
		if it.Days != 1 {
			unit = "days"
		}
		return fmt.Sprintf("%s — Day pass (%d %s)", it.CamperName, it.Days, unit)
	case notify.ItemAccommodation:
		line := it.CamperName + " — " + it.AccommodationName
		if it.UnitName != "" {
			line += " (" + it.UnitName + ")"
		}
		return line
	case notify.ItemDeposit:
		return fmt.Sprintf("%s — Camp deposit", it.CamperName)
	case notify.ItemCoach:
		return fmt.Sprintf("Coach transport (×%d)", it.Count)
	default:
		return it.CamperName
	}
}

func itemLines(items []notify.InvoiceItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = itemLine(it)
	}
	return out
}

func renderStaffAllocationReleased(p notify.StaffAllocationReleased) (subject, htmlBody string, err error) {
	subject = "Camp allocation released (unpaid)"
	body := fmt.Sprintf(
		"Group %s (%s) — allocation released.\nReason: %s\nCampers: %s",
		p.GroupID, p.ContactEmail, p.Reason, strings.Join(p.CamperNames, ", "))
	return renderWhiteTeamNotification(subject, body)
}

func renderWhiteTeamNotification(subject, body string) (string, string, error) {
	// body is plain text; HTML escaping applied in template helper
	escaped := template.HTMLEscapeString(body)
	return subject, fmt.Sprintf(`<!DOCTYPE html><html><body style="font-family:sans-serif;white-space:pre-wrap">%s</body></html>`, escaped), nil
}

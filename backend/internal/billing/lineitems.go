package billing

import "strings"

func StrVal(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

package domain

import "strings"

// ShirtSizeNotApplicable is the wire value day-pass holders submit when they're
// not buying a t-shirt. Per the form PDF, they should type "N/A".
const ShirtSizeNotApplicable = "n/a"

// ShirtSize is one selectable option in the camp t-shirt dropdown.
type ShirtSize struct {
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"` // "adult" | "child"
}

// shirtSizes is the canonical list pulled from the camp leader's spec. Codes
// are stable; display names can change without breaking submitted data.
var shirtSizes = []ShirtSize{
	{Code: "adult_s", DisplayName: "Small", Category: "adult"},
	{Code: "adult_m", DisplayName: "Medium", Category: "adult"},
	{Code: "adult_l", DisplayName: "Large", Category: "adult"},
	{Code: "adult_xl", DisplayName: "X-Large", Category: "adult"},
	{Code: "adult_2xl", DisplayName: "2X-Large", Category: "adult"},
	{Code: "child_0_6m", DisplayName: "0-6 months", Category: "child"},
	{Code: "child_6_12m", DisplayName: "6-12 months", Category: "child"},
	{Code: "child_12_18m", DisplayName: "12-18 months", Category: "child"},
	{Code: "child_18_24m", DisplayName: "18-24 months", Category: "child"},
	{Code: "child_2_3y", DisplayName: "2-3 years", Category: "child"},
	{Code: "child_3_4y", DisplayName: "3-4 years", Category: "child"},
	{Code: "child_5_6y", DisplayName: "5-6 years", Category: "child"},
	{Code: "child_7_8y", DisplayName: "7-8 years", Category: "child"},
	{Code: "child_9_11y", DisplayName: "9-11 years", Category: "child"},
	{Code: "child_12_13y", DisplayName: "12-13 years", Category: "child"},
	{Code: "child_14_15y", DisplayName: "14-15 years", Category: "child"},
}

var shirtSizeSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(shirtSizes))
	for _, s := range shirtSizes {
		m[s.Code] = struct{}{}
	}
	return m
}()

// ListShirtSizes returns the canonical t-shirt size options (does NOT include
// the "n/a" sentinel — that's a submit-time value, not a UI choice).
func ListShirtSizes() []ShirtSize { return shirtSizes }

// IsRealShirtSize reports whether code is one of the catalogued sizes
// (excluding the "n/a" sentinel). Comparison is case-insensitive.
func IsRealShirtSize(code string) bool {
	_, ok := shirtSizeSet[strings.ToLower(strings.TrimSpace(code))]
	return ok
}

// IsAcceptableShirtSize reports whether code is either a real size or the
// "n/a" sentinel.
func IsAcceptableShirtSize(code string) bool {
	c := strings.ToLower(strings.TrimSpace(code))
	if c == ShirtSizeNotApplicable {
		return true
	}
	_, ok := shirtSizeSet[c]
	return ok
}

package invoicing

// ExtendDueRequest is PATCH /camp-admin/registrations/{groupID}/invoice-due.
type ExtendDueRequest struct {
	DueAt           string `json:"due_at"`
	ExpectedVersion *int   `json:"expected_version,omitempty"`
}

// DeleteSummary captures Stripe cleanup performed when a registration is deleted.
type DeleteSummary struct {
	ContactName     string
	ContactEmail    string
	BalanceRefunded bool
	InvoiceVoided   bool
	AmountPence     int64
	RetainedPence   int64
}

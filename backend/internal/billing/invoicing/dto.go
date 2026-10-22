package invoicing

type ExtendDueRequest struct {
	DueAt string
}

type DeleteSummary struct {
	ContactName     string
	ContactEmail    string
	BalanceRefunded bool
	InvoiceVoided   bool
	AmountPence     int64
	RetainedPence   int64
}

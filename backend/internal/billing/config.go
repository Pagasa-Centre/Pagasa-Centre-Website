package billing

// Config holds billing/invoice settings from the app config.
type Config struct {
	StripePriceChildUnder3 string
	StripePriceDayPass     string
	StripePriceCoach       string
	InvoiceDueDays         int
	WhiteTeamEmail         string
}

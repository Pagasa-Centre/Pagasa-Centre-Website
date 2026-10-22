package billing

// Config holds billing/invoice settings from the app config.
type Config struct {
	ChildUnder3PriceRef string
	DayPassPriceRef     string
	CoachPriceRef       string
	DepositPricePence   int
	InvoiceDueDays      int
	WhiteTeamEmail      string
}

package stripe

import stripesdk "github.com/stripe/stripe-go/v85"

// Init sets the global Stripe API key used by BillingClient and PriceCatalog.
func Init(secretKey string) {
	stripesdk.Key = secretKey
}

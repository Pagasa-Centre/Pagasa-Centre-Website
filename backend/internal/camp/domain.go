package camp

import "time"

type Config struct {
	Name              string    `json:"name"`
	LocationName      string    `json:"location_name"`
	LocationAddr      string    `json:"location_addr"`
	WebsiteURL        string    `json:"website_url"`
	StartDate         time.Time `json:"start_date"`
	EndDate           time.Time `json:"end_date"`
	RegistrationsOpen bool      `json:"registrations_open"`
}

type Price struct {
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
	AmountPence int    `json:"amount_pence"`
	Currency    string `json:"currency"`
}

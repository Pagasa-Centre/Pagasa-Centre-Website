package domain

import "time"

type Config struct {
	Name                    string
	LocationName            string
	LocationAddr            string
	WebsiteURL              string
	StartDate               time.Time
	EndDate                 time.Time
	RegistrationsOpen       bool
	RegistrationPaymentMode string
}

type Price struct {
	Code        string
	DisplayName string
	AmountPence int
	Currency    string
}

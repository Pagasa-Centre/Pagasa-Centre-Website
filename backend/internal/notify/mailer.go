package notify

import (
	"context"
	"time"
)

type Mailer interface {
	SendDepositConfirmation(ctx context.Context, p DepositConfirmation) error
	SendAllocationReleased(ctx context.Context, p AllocationReleased) error
	SendStaffAllocationReleased(ctx context.Context, p StaffAllocationReleased) error
	SendBalanceInvoice(ctx context.Context, p BalanceInvoice) error
	SendBalancePaid(ctx context.Context, p BalancePaid) error
	SendBalancePaidConfirmation(ctx context.Context, p BalancePaidConfirmation) error
	SendSponsorshipConfirmed(ctx context.Context, p SponsorshipConfirmed) error
	SendAccommodationChanged(ctx context.Context, p AccommodationChangedNotice) error
	SendCoachInvoice(ctx context.Context, p CoachInvoice) error
	SendBookingUpdated(ctx context.Context, p BookingUpdated) error
}

type Amount struct {
	Pence    int64
	Currency string
}

type AccommodationChange struct {
	CamperName   string
	FirstChoice  string
	SecondChoice string
	Allocated    string
	TentGuidance bool
}

type DepositConfirmation struct {
	ToEmail        string
	ToName         string
	AmountPence    int
	Currency       string
	CamperCount    int
	HasMinor       bool
	ConsentFormURL string
	IsFree         bool
	PaidInFull     bool
}

type AllocationReleased struct {
	ToEmail     string
	ToName      string
	CamperNames []string
	Reason      string
	Cancelled   bool
}

type StaffAllocationReleased struct {
	ToEmail      string
	GroupID      string
	ContactEmail string
	Reason       string
	CamperNames  []string
}

type BalanceInvoice struct {
	ToEmail string
	ToName  string
	PayURL  string
	DueAt   *time.Time
	Amount  Amount
	Items   []InvoiceItem
	Changes []AccommodationChange
}

type CoachInvoice struct {
	ToEmail        string
	ToName         string
	PayURL         string
	DueAt          *time.Time
	Amount         Amount
	PassengerCount int
}

type BalancePaid struct {
	ToEmail      string
	ContactName  string
	ContactEmail string
	Amount       Amount
	PaidAt       time.Time
	Items        []InvoiceItem
}

type BalancePaidConfirmation struct {
	ToEmail string
	ToName  string
	Amount  Amount
	Items   []InvoiceItem
	Changes []AccommodationChange
}

type SponsorshipConfirmed struct {
	ToEmail string
	ToName  string
	Items   []InvoiceItem
	Changes []AccommodationChange
}

type AccommodationChangedNotice struct {
	ToEmail         string
	ToName          string
	Items           []AccommodationChange
	AwaitingPayment bool
}

type BookingUpdated struct {
	ToEmail          string
	ToName           string
	Items            []InvoiceItem
	AddedName        string
	InvoiceCancelled bool
}

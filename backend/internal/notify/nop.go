package notify

import "context"

type NopMailer struct{}

func (NopMailer) SendDepositConfirmation(context.Context, DepositConfirmation) error { return nil }
func (NopMailer) SendAllocationReleased(context.Context, AllocationReleased) error   { return nil }
func (NopMailer) SendStaffAllocationReleased(context.Context, StaffAllocationReleased) error {
	return nil
}
func (NopMailer) SendBalanceInvoice(context.Context, BalanceInvoice) error { return nil }
func (NopMailer) SendCoachInvoice(context.Context, CoachInvoice) error     { return nil }
func (NopMailer) SendBalancePaid(context.Context, BalancePaid) error       { return nil }
func (NopMailer) SendBalancePaidConfirmation(context.Context, BalancePaidConfirmation) error {
	return nil
}
func (NopMailer) SendSponsorshipConfirmed(context.Context, SponsorshipConfirmed) error {
	return nil
}
func (NopMailer) SendAccommodationChanged(context.Context, AccommodationChangedNotice) error {
	return nil
}
func (NopMailer) SendBookingUpdated(context.Context, BookingUpdated) error { return nil }

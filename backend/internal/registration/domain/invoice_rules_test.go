package domain

import (
	"testing"
)

func TestCanResendInvoice_noInvoiceOnFile(t *testing.T) {
	g := &Group{BillingStatus: BillingInvoiced, PaymentStatus: PaymentPaid, BalanceInvoiceRef: nil}
	_, err := g.CanResendInvoice()
	assertInvalidMsg(t, err, "no invoice on file")
}

func TestCanDelete_prepaid(t *testing.T) {
	g := &Group{PaidInFullAtRegistration: true, PaymentStatus: PaymentPaid}
	err := g.CanDelete()
	assertInvalidMsg(t, err, "this family paid for everything in one payment, so the deposit cannot be "+
		"separated from the rest automatically. Refund what they are owed in "+
		"Stripe first, then delete the registration.")
}

func TestAllocatedForInvoice(t *testing.T) {
	_, err := (Camper{FirstName: "A", LastName: "B"}).AllocatedForInvoice()
	assertInvalidMsg(t, err, "camper A B is not allocated")
	code := "tent"
	_, err = (Camper{FirstName: "A", LastName: "B", AllocatedAccommodationCode: &code}).AllocatedForInvoice()
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteRetention(t *testing.T) {
	g := &Group{PaymentStatus: PaymentPaid, TotalAmountPence: 5000}
	campers := []Camper{{DepositPaidPence: 500}}
	retained, onInv := DeleteRetention(g, campers)
	if retained != 5500 || onInv != 500 {
		t.Fatalf("retained=%d onInv=%d", retained, onInv)
	}
}

func TestContactName(t *testing.T) {
	g := Group{ContactFirstName: " Pat ", ContactLastName: " Lee "}
	if g.ContactName() != "Pat   Lee" {
		t.Fatalf("got %q", g.ContactName())
	}
}

package domain

import (
	"errors"
	"testing"

	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

var allBillingStatuses = []BillingStatus{
	BillingNone, BillingAllocated, BillingInvoiced,
	BillingBalancePaid, BillingReleased, BillingFreeConfirmed,
}

func strPtr(s string) *string { return &s }

// outcome is what a lifecycle method should produce: a target status, a
// sentinel error, or an *apperr.Error with an exact message.
type outcome struct {
	to      BillingStatus
	err     error
	message string
}

func checkOutcome(t *testing.T, g Group, tr BillingTransition, err error, want outcome) {
	t.Helper()
	switch {
	case want.err != nil:
		if !errors.Is(err, want.err) {
			t.Fatalf("err = %v, want %v", err, want.err)
		}
	case want.message != "":
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Kind != apperr.KindInvalid || ae.Message != want.message {
			t.Fatalf("err = %v, want invalid %q", err, want.message)
		}
	default:
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if tr.From != g.BillingStatus || tr.To != want.to {
			t.Fatalf("transition = %+v, want {%s %s}", tr, g.BillingStatus, want.to)
		}
	}
}

func TestAllocate(t *testing.T) {
	for _, status := range allBillingStatuses {
		for _, prepaid := range []bool{false, true} {
			g := Group{PaymentStatus: PaymentPaid, BillingStatus: status, PaidInFullAtRegistration: prepaid}
			want := outcome{to: BillingAllocated}
			switch {
			case status == BillingInvoiced:
				want = outcome{message: "cannot change allocation while invoice is open; release first"}
			case status == BillingBalancePaid && !prepaid:
				want = outcome{err: ErrBalancePaid}
			case prepaid:
				want = outcome{to: status}
			}
			t.Run(string(status), func(t *testing.T) {
				tr, err := g.Allocate()
				checkOutcome(t, g, tr, err, want)
			})
		}
	}
	g := Group{PaymentStatus: PaymentPending, BillingStatus: BillingNone}
	tr, err := g.Allocate()
	checkOutcome(t, g, tr, err, outcome{message: "deposit must be paid before allocation"})
}

func TestUnallocate(t *testing.T) {
	for _, status := range allBillingStatuses {
		for _, prepaid := range []bool{false, true} {
			g := Group{PaymentStatus: PaymentPaid, BillingStatus: status, PaidInFullAtRegistration: prepaid}
			want := outcome{to: BillingNone}
			switch {
			case status == BillingInvoiced:
				want = outcome{err: ErrInvoiceOpen}
			case status == BillingBalancePaid && !prepaid:
				want = outcome{err: ErrBalancePaid}
			case prepaid:
				want = outcome{to: status}
			}
			t.Run(string(status), func(t *testing.T) {
				tr, err := g.Unallocate()
				checkOutcome(t, g, tr, err, want)
			})
		}
	}
}

func TestInvoice(t *testing.T) {
	for _, status := range allBillingStatuses {
		for _, hasFullWeek := range []bool{false, true} {
			g := Group{PaymentStatus: PaymentPaid, BillingStatus: status}
			want := outcome{to: BillingInvoiced}
			switch status {
			case BillingBalancePaid:
				want = outcome{err: ErrBalancePaid}
			case BillingInvoiced:
				want = outcome{err: ErrInvoiceAlreadySent}
			case BillingNone, BillingReleased:
				if hasFullWeek {
					want = outcome{message: "group must be allocated before invoicing"}
				}
			}
			t.Run(string(status), func(t *testing.T) {
				tr, err := g.Invoice(hasFullWeek)
				checkOutcome(t, g, tr, err, want)
			})
		}
	}

	cases := map[string]struct {
		g    Group
		want outcome
	}{
		"sponsored": {
			Group{IsFree: true, PaymentStatus: PaymentPaid, BillingStatus: BillingAllocated},
			outcome{message: "this is a church-sponsored registration; confirm the sponsorship instead of invoicing"},
		},
		"paid in full": {
			Group{PaidInFullAtRegistration: true, PaymentStatus: PaymentPaid, BillingStatus: BillingBalancePaid},
			outcome{message: "this family paid in full at registration; there is nothing to invoice"},
		},
		"deposit unpaid": {
			Group{PaymentStatus: PaymentPending, BillingStatus: BillingAllocated},
			outcome{err: ErrDepositUnpaid},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tr, err := tc.g.Invoice(true)
			checkOutcome(t, tc.g, tr, err, tc.want)
		})
	}
}

func TestConfirmSponsorship(t *testing.T) {
	for _, status := range allBillingStatuses {
		g := Group{IsFree: true, PaymentStatus: PaymentPaid, BillingStatus: status}
		want := outcome{message: "group must be allocated before confirming sponsorship"}
		if status == BillingAllocated {
			want = outcome{to: BillingFreeConfirmed}
		}
		t.Run(string(status), func(t *testing.T) {
			tr, err := g.ConfirmSponsorship()
			checkOutcome(t, g, tr, err, want)
		})
	}

	g := Group{PaymentStatus: PaymentPaid, BillingStatus: BillingAllocated}
	tr, err := g.ConfirmSponsorship()
	checkOutcome(t, g, tr, err, outcome{message: "this group is not church-sponsored"})

	g = Group{IsFree: true, PaymentStatus: PaymentPending, BillingStatus: BillingAllocated}
	tr, err = g.ConfirmSponsorship()
	checkOutcome(t, g, tr, err, outcome{err: ErrDepositUnpaid})
}

func TestMarkBalancePaidManually(t *testing.T) {
	for _, status := range allBillingStatuses {
		g := Group{PaymentStatus: PaymentPaid, BillingStatus: status}
		want := outcome{message: "only allocated or invoiced groups can be marked paid; this group is " + status.Label()}
		if status == BillingAllocated || status == BillingInvoiced {
			want = outcome{to: BillingBalancePaid}
		}
		t.Run(string(status), func(t *testing.T) {
			tr, err := g.MarkBalancePaidManually()
			checkOutcome(t, g, tr, err, want)
		})
	}

	g := Group{PaymentStatus: PaymentPending, BillingStatus: BillingAllocated}
	tr, err := g.MarkBalancePaidManually()
	checkOutcome(t, g, tr, err, outcome{err: ErrDepositUnpaid})

	g = Group{IsFree: true, PaymentStatus: PaymentPaid, BillingStatus: BillingAllocated}
	tr, err = g.MarkBalancePaidManually()
	checkOutcome(t, g, tr, err, outcome{message: "church-sponsored groups are settled with Confirm sponsorship"})
}

func TestRelease(t *testing.T) {
	for _, status := range allBillingStatuses {
		g := Group{BillingStatus: status}
		tr := g.Release()
		if tr.From != status || tr.To != BillingReleased {
			t.Fatalf("%s: transition = %+v", status, tr)
		}
	}
}

func TestSettleBalance(t *testing.T) {
	for _, status := range allBillingStatuses {
		g := Group{BillingStatus: status}
		tr, already := g.SettleBalance()
		if status == BillingBalancePaid {
			if !already || tr != (BillingTransition{}) {
				t.Fatalf("balance_paid: tr=%+v already=%v", tr, already)
			}
			continue
		}
		if already || tr.From != status || tr.To != BillingBalancePaid {
			t.Fatalf("%s: tr=%+v already=%v", status, tr, already)
		}
	}
}

func TestRequireOpenInvoice(t *testing.T) {
	for _, status := range allBillingStatuses {
		g := Group{BillingStatus: status}
		want := outcome{message: "invoice is not open"}
		if status == BillingInvoiced {
			want = outcome{to: BillingInvoiced}
		}
		t.Run(string(status), func(t *testing.T) {
			tr, err := g.RequireOpenInvoice()
			checkOutcome(t, g, tr, err, want)
			if err == nil && tr.ChangesBilling() {
				t.Fatal("RequireOpenInvoice must not change billing_status")
			}
		})
	}
}

func TestHasOpenInvoiceAndBalanceSettled(t *testing.T) {
	inv := "in_1"
	blank := "  "
	cases := []struct {
		g        Group
		open     bool
		balanced bool
	}{
		{Group{BillingStatus: BillingInvoiced, StripeInvoiceID: &inv}, true, false},
		{Group{BillingStatus: BillingInvoiced, StripeInvoiceID: &blank}, false, false},
		{Group{BillingStatus: BillingInvoiced}, false, false},
		{Group{BillingStatus: BillingAllocated, StripeInvoiceID: &inv}, false, false},
		{Group{BillingStatus: BillingBalancePaid, StripeInvoiceID: &inv}, false, true},
	}
	for _, tc := range cases {
		if got := tc.g.HasOpenInvoice(); got != tc.open {
			t.Fatalf("%+v HasOpenInvoice = %v", tc.g.BillingStatus, got)
		}
		if got := tc.g.BalanceSettled(); got != tc.balanced {
			t.Fatalf("%+v BalanceSettled = %v", tc.g.BillingStatus, got)
		}
	}
}

func TestRosterChanged(t *testing.T) {
	inv := "in_1"
	placed := []Camper{
		{AttendanceType: AttendanceFullWeek, AllocatedAccommodationCode: strPtr("lodge")},
		{AttendanceType: AttendanceDayPass},
	}
	unplaced := []Camper{
		{AttendanceType: AttendanceFullWeek, AllocatedAccommodationCode: strPtr("lodge")},
		{AttendanceType: AttendanceFullWeek},
	}
	dayPassOnly := []Camper{{AttendanceType: AttendanceDayPass}}

	cases := map[string]struct {
		g      Group
		after  []Camper
		stale  bool
		wantTo BillingStatus // empty means no billing change
		void   bool
	}{
		"invoiced stale all placed":         {Group{BillingStatus: BillingInvoiced, StripeInvoiceID: &inv}, placed, true, BillingAllocated, true},
		"invoiced stale full week unplaced": {Group{BillingStatus: BillingInvoiced, StripeInvoiceID: &inv}, unplaced, true, BillingNone, true},
		"invoiced stale day pass only":      {Group{BillingStatus: BillingInvoiced, StripeInvoiceID: &inv}, dayPassOnly, true, BillingNone, true},
		"invoiced not stale":                {Group{BillingStatus: BillingInvoiced, StripeInvoiceID: &inv}, unplaced, false, "", false},
		"allocated full week unplaced":      {Group{BillingStatus: BillingAllocated}, unplaced, false, BillingNone, false},
		"allocated all placed":              {Group{BillingStatus: BillingAllocated}, placed, true, "", false},
		"allocated day pass only":           {Group{BillingStatus: BillingAllocated}, dayPassOnly, false, "", false},
		"none":                              {Group{BillingStatus: BillingNone}, unplaced, true, "", false},
		"balance paid":                      {Group{BillingStatus: BillingBalancePaid}, unplaced, true, "", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tr, void := tc.g.RosterChanged(tc.after, tc.stale)
			if void != tc.void {
				t.Fatalf("voidInvoice = %v, want %v", void, tc.void)
			}
			if tc.wantTo == "" {
				if tr.ChangesBilling() {
					t.Fatalf("unexpected transition %+v", tr)
				}
				return
			}
			if tr.From != tc.g.BillingStatus || tr.To != tc.wantTo {
				t.Fatalf("transition = %+v, want {%s %s}", tr, tc.g.BillingStatus, tc.wantTo)
			}
		})
	}
}

func TestAllocationIsEdit(t *testing.T) {
	placed := []Camper{{AttendanceType: AttendanceFullWeek, AllocatedAccommodationCode: strPtr("lodge")}}
	cases := map[string]struct {
		g       Group
		campers []Camper
		want    bool
	}{
		"allocated":                  {Group{BillingStatus: BillingAllocated}, nil, true},
		"none first allocation":      {Group{BillingStatus: BillingNone}, placed, false},
		"prepaid with placement":     {Group{BillingStatus: BillingBalancePaid, PaidInFullAtRegistration: true}, placed, true},
		"prepaid without placement":  {Group{BillingStatus: BillingBalancePaid, PaidInFullAtRegistration: true}, []Camper{{AttendanceType: AttendanceFullWeek}}, false},
		"prepaid day pass placement": {Group{BillingStatus: BillingBalancePaid, PaidInFullAtRegistration: true}, []Camper{{AttendanceType: AttendanceDayPass, AllocatedAccommodationCode: strPtr("lodge")}}, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := tc.g.AllocationIsEdit(tc.campers); got != tc.want {
				t.Fatalf("AllocationIsEdit = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSettleCheckout(t *testing.T) {
	for _, ps := range []PaymentStatus{PaymentPending, PaymentPaid, PaymentFailed, PaymentFailedCapacity, PaymentRefunded, PaymentCancelled} {
		for _, prepaid := range []bool{false, true} {
			g := Group{PaymentStatus: ps, BillingStatus: BillingNone, PaidInFullAtRegistration: prepaid}
			s, ok := g.SettleCheckout()
			if ps != PaymentPending {
				if ok {
					t.Fatalf("%s: settled a non-pending group", ps)
				}
				continue
			}
			if !ok || s.Payment != (PaymentTransition{From: PaymentPending, To: PaymentPaid}) {
				t.Fatalf("pending: settlement = %+v ok=%v", s, ok)
			}
			wantBilling := BillingTransition{}
			if prepaid {
				wantBilling = BillingTransition{From: BillingNone, To: BillingBalancePaid}
			}
			if s.Billing != wantBilling {
				t.Fatalf("prepaid=%v: billing = %+v, want %+v", prepaid, s.Billing, wantBilling)
			}
		}
	}
}

func TestExpireCheckout(t *testing.T) {
	for _, ps := range []PaymentStatus{PaymentPending, PaymentPaid, PaymentCancelled} {
		g := Group{PaymentStatus: ps}
		tr, ok := g.ExpireCheckout()
		if ps == PaymentPending {
			if !ok || tr != (PaymentTransition{From: PaymentPending, To: PaymentCancelled}) {
				t.Fatalf("pending: tr=%+v ok=%v", tr, ok)
			}
		} else if ok {
			t.Fatalf("%s: expired a non-pending group", ps)
		}
	}
}

func TestOverridePaymentStatus(t *testing.T) {
	g := Group{PaymentStatus: PaymentPaid}
	tr := g.OverridePaymentStatus(PaymentRefunded)
	if tr != (PaymentTransition{From: PaymentPaid, To: PaymentRefunded}) {
		t.Fatalf("tr = %+v", tr)
	}
}

func TestParsePaymentStatus(t *testing.T) {
	for _, s := range []string{"pending", "paid", "failed", "failed_capacity", "refunded", "cancelled"} {
		if got, ok := ParsePaymentStatus(s); !ok || string(got) != s {
			t.Fatalf("ParsePaymentStatus(%q) = %q, %v", s, got, ok)
		}
	}
	if _, ok := ParsePaymentStatus("bogus"); ok {
		t.Fatal("bogus status accepted")
	}
}

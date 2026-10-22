package domain

import (
	"fmt"
	"strings"
)

type ChargeLine struct {
	Description string
	AmountPence int64
	Quantity    int64
}

type Charge struct {
	TotalPence int
	Currency   string
	Lines      []ChargeLine
}

type ChargeInputs struct {
	Mode                string
	Deposit             PriceRow
	HasCatalog          bool
	Tiers               []AccommodationType
	ChildUnder3PriceRef string
	DayPassPriceRef     string
	CoachPriceRef       string
}

type PriceAmountFunc func(priceRef string) (amountPence int64, currency string, err error)

func ComputeCharge(req Submission, in ChargeInputs, amount PriceAmountFunc) (Charge, error) {
	currency := in.Deposit.Currency
	if currency == "" {
		currency = "GBP"
	}

	if in.Mode != PaymentModeFull {
		paying := req.DepositPayingCount()
		total := in.Deposit.AmountPence * paying
		var lines []ChargeLine
		if total > 0 {
			lines = []ChargeLine{{
				Description: fmt.Sprintf("PC Summer Camp 2026 non-refundable deposit (%d camper%s)", paying, chargePluralS(paying)),
				AmountPence: int64(total),
				Quantity:    1,
			}}
		}
		return Charge{TotalPence: total, Currency: currency, Lines: lines}, nil
	}

	if !in.HasCatalog {
		return Charge{}, fmt.Errorf("stripe price catalog not configured")
	}

	var lines []ChargeLine
	totalPence := 0

	paying := req.DepositPayingCount()
	if paying > 0 && in.Deposit.AmountPence > 0 {
		lines = append(lines, ChargeLine{
			Description: "PC Summer Camp 2026 non-refundable deposit",
			AmountPence: int64(in.Deposit.AmountPence),
			Quantity:    int64(paying),
		})
		totalPence += in.Deposit.AmountPence * paying
	}

	tierByCode := map[string]AccommodationType{}
	for _, t := range in.Tiers {
		tierByCode[t.Code] = t
	}

	type accKey struct {
		priceID     string
		description string
		unitPence   int64
	}
	accCounts := map[accKey]int64{}

	for _, c := range req.Campers {
		if c.Attendance.Type != AttendanceFullWeek {
			continue
		}
		code := strings.TrimSpace(c.Attendance.AccommodationFirstChoice)
		if code == "" {
			return Charge{}, fmt.Errorf("full-week camper missing accommodation first choice")
		}
		tier := tierByCode[code]
		priceID, err := AccommodationPriceID(code, c.Age, &tier, in.ChildUnder3PriceRef)
		if err != nil {
			return Charge{}, err
		}
		unit, cur, err := amount(priceID)
		if err != nil {
			return Charge{}, fmt.Errorf("lookup price for %s: %w", code, err)
		}
		if err := ensureChargeCurrency(currency, cur); err != nil {
			return Charge{}, err
		}
		if unit <= 0 {
			continue
		}
		desc := fmt.Sprintf("%s — full week", tier.DisplayName)
		k := accKey{priceID: priceID, description: desc, unitPence: unit}
		accCounts[k]++
	}

	for k, qty := range accCounts {
		lines = append(lines, ChargeLine{
			Description: k.description,
			AmountPence: k.unitPence,
			Quantity:    qty,
		})
		totalPence += int(k.unitPence * qty)
	}

	dayPassDays := 0
	for _, c := range req.Campers {
		if c.Attendance.Type == AttendanceDayPass {
			dayPassDays += len(c.Attendance.Days)
		}
	}
	if dayPassDays > 0 {
		if in.DayPassPriceRef == "" {
			return Charge{}, fmt.Errorf("STRIPE_PRICE_DAY_PASS is not configured")
		}
		unit, cur, err := amount(in.DayPassPriceRef)
		if err != nil {
			return Charge{}, fmt.Errorf("lookup day pass price: %w", err)
		}
		if err := ensureChargeCurrency(currency, cur); err != nil {
			return Charge{}, err
		}
		if unit > 0 {
			lines = append(lines, ChargeLine{
				Description: "PC Summer Camp 2026 day pass",
				AmountPence: unit,
				Quantity:    int64(dayPassDays),
			})
			totalPence += int(unit * int64(dayPassDays))
		}
	}

	coachCount := req.CoachEligibleCount()
	if coachCount > 0 {
		if in.CoachPriceRef == "" {
			return Charge{}, fmt.Errorf("STRIPE_PRICE_COACH is not configured")
		}
		unit, cur, err := amount(in.CoachPriceRef)
		if err != nil {
			return Charge{}, fmt.Errorf("lookup coach price: %w", err)
		}
		if err := ensureChargeCurrency(currency, cur); err != nil {
			return Charge{}, err
		}
		if unit > 0 {
			lines = append(lines, ChargeLine{
				Description: "PC Summer Camp 2026 coach travel",
				AmountPence: unit,
				Quantity:    coachCount,
			})
			totalPence += int(unit * coachCount)
		}
	}

	return Charge{TotalPence: totalPence, Currency: currency, Lines: lines}, nil
}

func ensureChargeCurrency(expected, got string) error {
	if strings.EqualFold(expected, got) {
		return nil
	}
	return fmt.Errorf("price currency mismatch: expected %s, got %s", expected, got)
}

func chargePluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

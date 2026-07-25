// Pure helpers for the camp registration form. Mirrors the backend pricing
// logic in backend/internal/registration/service.go::computeTotal so the UI
// running-total stays consistent with what Stripe gets charged.

import type { CamperSubmission, DayCode, Price, RegistrationPricing } from "@/lib/api";

export const SHIRT_SIZE_NOT_APPLICABLE = "n/a";

// Youngest age that has to pay the deposit. Campers under this attend free
// (cot / lap-of-parent). Mirrors backend MinDepositAge.
export const MIN_DEPOSIT_AGE = 4;

// Code for "Child accommodation (sharing with parent)". When chosen as the
// 1st preference, the 2nd-choice picker is hidden because a child is by
// definition with their parent — there's no meaningful fallback to pick.
// Mirrors backend AccommodationChild.
export const ACCOMMODATION_CHILD_CODE = "child";

// Code for "Tent (bring your own)". When chosen as the 1st preference the
// 2nd-choice picker is hidden/optional: tents have no hard cap, so there's no
// realistic chance of being bumped to a fallback tier. Mirrors backend
// AccommodationTent.
export const ACCOMMODATION_TENT_CODE = "tent";

// Oldest age that can pick the child-with-parent accommodation. Anyone older
// has to pick a regular tier (lodge/cabin/etc.). Mirrors backend
// MaxChildAccommodationAge.
export const MAX_CHILD_ACCOMMODATION_AGE = 12;

/**
 * CamperState is the raw form-state shape for a single camper. Strings (not
 * numbers/enums) so partially-typed inputs are easy to handle. We convert to
 * the strict CamperSubmission shape at submit time.
 */
export type CamperState = {
  first_name: string;
  last_name: string;
  gender: "" | "male" | "female";
  age: string;
  cell_leader_name: string;
  is_cell_leader: boolean;
  attendance_type: "" | "full_week" | "day_pass";
  // full_week fields
  shirt_size: string;
  dietary_requirements: string;
  needs_coach: boolean | null;
  accommodation_first_choice: string;
  accommodation_second_choice: string;
  roommate_requests: string;
  // day_pass fields
  day_pass_days: DayCode[];
  tshirt_option: "" | "team_activities" | "tshirt_only" | "none";
  needs_catering: boolean | null;
};

export function emptyCamper(): CamperState {
  return {
    first_name: "",
    last_name: "",
    gender: "",
    age: "",
    cell_leader_name: "",
    is_cell_leader: false,
    attendance_type: "",
    shirt_size: "",
    dietary_requirements: "",
    needs_coach: null,
    accommodation_first_choice: "",
    accommodation_second_choice: "",
    roommate_requests: "",
    day_pass_days: [],
    tshirt_option: "",
    needs_catering: null,
  };
}

export const DAY_PASS_DAYS: { code: DayCode; label: string }[] = [
  { code: "mon", label: "Day 1 — Monday 10 Aug" },
  { code: "tue", label: "Day 2 — Tuesday 11 Aug" },
  { code: "wed", label: "Day 3 — Wednesday 12 Aug" },
  { code: "thu", label: "Day 4 — Thursday 13 Aug" },
  { code: "fri", label: "Day 5 — Friday 14 Aug" },
];

export function isMinor(age: number): boolean {
  return Number.isFinite(age) && age > 0 && age < 18;
}

export function formatPence(amountPence: number, currency = "GBP"): string {
  try {
    return new Intl.NumberFormat("en-GB", {
      style: "currency",
      currency,
    }).format(amountPence / 100);
  } catch {
    return `${currency} ${(amountPence / 100).toFixed(2)}`;
  }
}

function priceMap(prices: Price[]): Record<string, Price> {
  const m: Record<string, Price> = {};
  for (const p of prices) m[p.code] = p;
  return m;
}

/**
 * computeTotalPence mirrors the backend pricing logic exactly:
 *
 * - full_week + age >= MIN_DEPOSIT_AGE: one flat "deposit" charge per camper
 * - full_week + age <  MIN_DEPOSIT_AGE: free (cot / lap-of-parent)
 * - day_pass:                            £0 at registration
 *
 * Missing price codes contribute 0 (we surface this via a "prices not yet set"
 * notice in the form rather than failing silently).
 */
export function computeTotalPence(
  campers: CamperSubmission[],
  prices: Price[],
): number {
  const lookup = priceMap(prices);
  const deposit = lookup.deposit?.amount_pence ?? 0;
  let total = 0;
  for (const c of campers) {
    if (c.attendance.type === "full_week" && c.age >= MIN_DEPOSIT_AGE) {
      total += deposit;
    }
    // Day-pass campers and under-3s contribute nothing at registration time.
  }
  return total;
}

export function payingForDeposit(c: CamperSubmission): boolean {
  return c.attendance.type === "full_week" && c.age >= MIN_DEPOSIT_AGE;
}

export function isUnderDepositAge(age: number): boolean {
  return Number.isFinite(age) && age > 0 && age < MIN_DEPOSIT_AGE;
}

export function pricesCurrency(prices: Price[]): string {
  return prices[0]?.currency ?? "GBP";
}

function tierAmount(
  pricing: RegistrationPricing,
  code: string,
  age: number,
): number {
  if (code === ACCOMMODATION_CHILD_CODE && age < MIN_DEPOSIT_AGE) {
    return pricing.child_under3_amount_pence;
  }
  return (
    pricing.accommodation_tiers.find((t) => t.code === code)?.amount_pence ?? 0
  );
}

/**
 * computeFullTotalPence mirrors backend full-payment mode pricing.
 */
export function computeFullTotalPence(
  campers: CamperSubmission[],
  pricing: RegistrationPricing,
): number {
  let total = 0;
  for (const c of campers) {
    if (c.attendance.type === "full_week" && c.age >= MIN_DEPOSIT_AGE) {
      total += pricing.deposit_amount_pence;
    }
    if (c.attendance.type === "full_week") {
      const code = c.attendance.accommodation_first_choice;
      if (code) {
        total += tierAmount(pricing, code, c.age);
      }
    }
    if (c.attendance.type === "day_pass") {
      total += pricing.day_pass_amount_pence * c.attendance.days.length;
    }
    if (
      c.attendance.type === "full_week" &&
      c.attendance.needs_coach &&
      c.age >= MIN_DEPOSIT_AGE
    ) {
      total += pricing.coach_amount_pence;
    }
  }
  return total;
}

export type FullPaymentBreakdown = {
  depositTotal: number;
  accommodationLines: { label: string; amount: number }[];
  dayPassTotal: number;
  coachTotal: number;
};

export function fullPaymentBreakdown(
  campers: CamperSubmission[],
  pricing: RegistrationPricing,
  accName: (code: string) => string,
): FullPaymentBreakdown {
  let depositTotal = 0;
  const accMap = new Map<string, { label: string; amount: number; count: number }>();
  let dayPassTotal = 0;
  let coachTotal = 0;

  for (const c of campers) {
    if (c.attendance.type === "full_week" && c.age >= MIN_DEPOSIT_AGE) {
      depositTotal += pricing.deposit_amount_pence;
    }
    if (c.attendance.type === "full_week") {
      const code = c.attendance.accommodation_first_choice;
      if (code) {
        const amt = tierAmount(pricing, code, c.age);
        if (amt > 0) {
          const label = `${accName(code)} — full week`;
          const existing = accMap.get(label);
          if (existing) {
            existing.count++;
            existing.amount += amt;
          } else {
            accMap.set(label, { label, amount: amt, count: 1 });
          }
        }
      }
    }
    if (c.attendance.type === "day_pass") {
      dayPassTotal +=
        pricing.day_pass_amount_pence * c.attendance.days.length;
    }
    if (
      c.attendance.type === "full_week" &&
      c.attendance.needs_coach &&
      c.age >= MIN_DEPOSIT_AGE
    ) {
      coachTotal += pricing.coach_amount_pence;
    }
  }

  const accommodationLines = [...accMap.values()].map((v) => ({
    label: v.count > 1 ? `${v.label} × ${v.count}` : v.label,
    amount: v.amount,
  }));

  return { depositTotal, accommodationLines, dayPassTotal, coachTotal };
}

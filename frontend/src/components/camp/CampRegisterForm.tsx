"use client";

import { useMemo, useState } from "react";
import {
  type Accommodation,
  type CampConfig,
  type CamperSubmission,
  type Price,
  type ShirtSize,
  type SubmitRequest,
  ApiClientError,
  camp,
} from "@/lib/api";
import {
  ACCOMMODATION_CHILD_CODE,
  type CamperState,
  computeTotalPence,
  emptyCamper,
  formatPence,
  isMinor,
  MAX_CHILD_ACCOMMODATION_AGE,
  MIN_DEPOSIT_AGE,
  payingForDeposit,
  pricesCurrency,
  SHIRT_SIZE_NOT_APPLICABLE,
} from "@/lib/camp";
import CamperFieldset from "./CamperFieldset";

export type SuccessStash = {
  group_id: string;
  has_minor: boolean;
  consent_form_url: string | null;
  contact_email: string;
  // Camper list for the "we've registered the following" panel on the
  // success page. We snapshot it client-side so the free path doesn't need
  // a backend round-trip; the Stripe path falls back to the backend
  // `/api/registrations/summary` endpoint if sessionStorage is missing.
  campers: { first_name: string; last_name: string }[];
};

const STASH_KEY = "pc-camp-last-registration";

type Props = {
  config: CampConfig;
  prices: Price[];
  accommodations: Accommodation[];
  shirtSizes: ShirtSize[];
};

type FormState = {
  contact: { email: string; phone: string };
  campers: CamperState[];
};

function initialState(): FormState {
  return { contact: { email: "", phone: "" }, campers: [emptyCamper()] };
}

const labelCls =
  "text-xs font-bold uppercase tracking-widest text-neutral-700";
const baseInput =
  "px-4 py-3 bg-white border text-neutral-900 placeholder-neutral-500 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary";

function inputCls(hasError: boolean) {
  return `${baseInput} ${hasError ? "border-red-500" : "border-neutral-300"}`;
}

function toSubmission(
  state: FormState,
): { ok: true; payload: SubmitRequest } | { ok: false; error: string } {
  if (state.campers.length === 0) {
    return { ok: false, error: "Please add at least one camper." };
  }
  const main = state.campers[0];
  if (!main.first_name || !main.last_name) {
    return {
      ok: false,
      error: "Main contact name is required.",
    };
  }

  const campers: CamperSubmission[] = [];
  for (let i = 0; i < state.campers.length; i++) {
    const c = state.campers[i];
    const age = parseInt(c.age, 10);
    if (!c.gender || (c.gender !== "male" && c.gender !== "female")) {
      return { ok: false, error: `Camper ${i + 1}: please select a gender.` };
    }
    if (!Number.isFinite(age) || age <= 0 || age >= 120) {
      return {
        ok: false,
        error: `Camper ${i + 1}: please enter a valid age (1–119).`,
      };
    }
    if (c.attendance_type !== "full_week" && c.attendance_type !== "day_pass") {
      return {
        ok: false,
        error: `Camper ${i + 1}: please choose Full week or Day pass.`,
      };
    }

    const base = {
      first_name: c.first_name.trim(),
      last_name: c.last_name.trim(),
      gender: c.gender,
      age,
      cell_leader_name: c.cell_leader_name.trim(),
      is_cell_leader: c.is_cell_leader,
      is_main_contact: i === 0,
    };

    if (c.attendance_type === "full_week") {
      if (c.needs_coach === null) {
        return {
          ok: false,
          error: `Camper ${i + 1}: please choose a transport option.`,
        };
      }
      if (!c.accommodation_first_choice) {
        return {
          ok: false,
          error: `Camper ${i + 1}: please pick a 1st accommodation choice.`,
        };
      }
      const isChildSharing =
        c.accommodation_first_choice === ACCOMMODATION_CHILD_CODE;
      if (
        (c.accommodation_first_choice === ACCOMMODATION_CHILD_CODE ||
          c.accommodation_second_choice === ACCOMMODATION_CHILD_CODE) &&
        age > MAX_CHILD_ACCOMMODATION_AGE
      ) {
        return {
          ok: false,
          error: `Camper ${i + 1}: child accommodation is only available for campers aged ${MAX_CHILD_ACCOMMODATION_AGE} or under.`,
        };
      }
      if (!isChildSharing && !c.accommodation_second_choice) {
        return {
          ok: false,
          error: `Camper ${i + 1}: please pick a 2nd accommodation choice.`,
        };
      }
      if (
        !isChildSharing &&
        c.accommodation_first_choice === c.accommodation_second_choice
      ) {
        return {
          ok: false,
          error: `Camper ${i + 1}: 1st and 2nd accommodation choices must differ.`,
        };
      }
      campers.push({
        ...base,
        attendance: {
          type: "full_week",
          shirt_size: c.shirt_size,
          dietary_requirements: c.dietary_requirements.trim(),
          needs_coach: c.needs_coach,
          accommodation_first_choice: c.accommodation_first_choice,
          accommodation_second_choice: c.accommodation_second_choice,
          roommate_requests: c.roommate_requests.trim(),
        },
      });
    } else {
      if (c.tshirt_option === "") {
        return {
          ok: false,
          error: `Camper ${i + 1}: please choose a t-shirt option.`,
        };
      }
      if (c.needs_catering === null) {
        return {
          ok: false,
          error: `Camper ${i + 1}: please choose a catering option.`,
        };
      }
      campers.push({
        ...base,
        attendance: {
          type: "day_pass",
          days: c.day_pass_days,
          tshirt_option: c.tshirt_option,
          shirt_size:
            c.tshirt_option === "none"
              ? SHIRT_SIZE_NOT_APPLICABLE
              : c.shirt_size,
          needs_catering: c.needs_catering,
          dietary_requirements: c.dietary_requirements.trim(),
        },
      });
    }
  }

  return {
    ok: true,
    payload: {
      contact: {
        first_name: main.first_name.trim(),
        last_name: main.last_name.trim(),
        email: state.contact.email.trim(),
        phone: state.contact.phone.trim(),
      },
      campers,
    },
  };
}

export default function CampRegisterForm({
  prices,
  accommodations,
  shirtSizes,
}: Props) {
  const [state, setState] = useState<FormState>(initialState);
  const [submitting, setSubmitting] = useState(false);
  const [topError, setTopError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  function updateContact(patch: Partial<FormState["contact"]>) {
    setState((s) => ({ ...s, contact: { ...s.contact, ...patch } }));
  }

  function updateCamper(index: number, patch: Partial<CamperState>) {
    setState((s) => ({
      ...s,
      campers: s.campers.map((c, i) => (i === index ? { ...c, ...patch } : c)),
    }));
  }

  function addCamper() {
    setState((s) => ({ ...s, campers: [...s.campers, emptyCamper()] }));
  }

  function removeCamper(index: number) {
    setState((s) => ({
      ...s,
      campers: s.campers.filter((_, i) => i !== index),
    }));
  }

  // Build a "preview" submission for the running total. We use a forgiving
  // version that just turns invalid campers into 0-contributing entries.
  const previewCampers = useMemo<CamperSubmission[]>(() => {
    return state.campers.flatMap((c, i): CamperSubmission[] => {
      const age = parseInt(c.age, 10);
      if (!Number.isFinite(age) || c.attendance_type === "") return [];
      const base = {
        first_name: c.first_name,
        last_name: c.last_name,
        gender: (c.gender || "male") as "male" | "female",
        age,
        cell_leader_name: c.cell_leader_name,
        is_cell_leader: c.is_cell_leader,
        is_main_contact: i === 0,
      };
      if (c.attendance_type === "full_week") {
        return [
          {
            ...base,
            attendance: {
              type: "full_week",
              shirt_size: c.shirt_size,
              dietary_requirements: c.dietary_requirements,
              needs_coach: c.needs_coach ?? false,
              accommodation_first_choice: c.accommodation_first_choice,
              accommodation_second_choice: c.accommodation_second_choice,
              roommate_requests: c.roommate_requests,
            },
          },
        ];
      }
      if (c.tshirt_option === "") return [];
      return [
        {
          ...base,
          attendance: {
            type: "day_pass",
            days: c.day_pass_days,
            tshirt_option: c.tshirt_option as
              | "team_activities"
              | "tshirt_only"
              | "none",
            shirt_size: c.shirt_size,
            needs_catering: c.needs_catering ?? false,
            dietary_requirements: c.dietary_requirements,
          },
        },
      ];
    });
  }, [state.campers]);

  const totalPence = useMemo(
    () => computeTotalPence(previewCampers, prices),
    [previewCampers, prices],
  );
  const currency = pricesCurrency(prices);
  const payingCamperCount = previewCampers.filter(payingForDeposit).length;
  const freeFullWeekCount = previewCampers.filter(
    (c) =>
      c.attendance.type === "full_week" && c.age < MIN_DEPOSIT_AGE,
  ).length;
  const dayPassCamperCount = previewCampers.filter(
    (c) => c.attendance.type === "day_pass",
  ).length;
  const depositPence = prices.find((p) => p.code === "deposit")?.amount_pence ?? 0;

  const anyMinor = state.campers.some((c) => {
    const age = parseInt(c.age, 10);
    return Number.isFinite(age) && isMinor(age);
  });

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setTopError(null);
    setFieldErrors({});

    const built = toSubmission(state);
    if (!built.ok) {
      setTopError(built.error);
      window.scrollTo({ top: 0, behavior: "smooth" });
      return;
    }

    setSubmitting(true);
    try {
      const res = await camp.submit(built.payload);
      const stash: SuccessStash = {
        group_id: res.group_id,
        has_minor: res.has_minor,
        consent_form_url: res.consent_form_url ?? null,
        contact_email: built.payload.contact.email,
        campers: built.payload.campers.map((c) => ({
          first_name: c.first_name,
          last_name: c.last_name,
        })),
      };
      try {
        sessionStorage.setItem(STASH_KEY, JSON.stringify(stash));
      } catch {
        // sessionStorage can be disabled — proceed anyway
      }
      if (res.checkout_url) {
        window.location.href = res.checkout_url;
      } else {
        // £0 total (day-pass-only): backend already marked paid + emailed.
        // Send the user straight to the success page; the ?free=1 flag lets
        // it adjust copy to match. Also pass group_id so the success page
        // can fall back to /api/registrations/summary if sessionStorage is
        // unavailable (rare, but possible).
        window.location.href = `/camp/registration/success?free=1&group_id=${encodeURIComponent(res.group_id)}`;
      }
    } catch (err) {
      setSubmitting(false);
      if (err instanceof ApiClientError) {
        setTopError(err.detail.message);
        setFieldErrors(err.detail.fields ?? {});
      } else {
        setTopError("Something went wrong. Please try again.");
      }
      window.scrollTo({ top: 0, behavior: "smooth" });
    }
  }

  return (
    <section className="bg-surface py-16 lg:py-20">
      <div className="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-10">
          <div className="w-14 h-1 bg-neutral-800 mb-7 mx-auto" />
          <p className="text-primary uppercase tracking-widest text-sm font-semibold mb-3">
            Registration
          </p>
          <h2 className="text-3xl sm:text-4xl font-extrabold text-neutral-900 leading-tight">
            Sign up for camp
          </h2>
          <p className="mt-4 text-neutral-600 max-w-xl mx-auto text-sm">
            Full-week campers pay a flat{" "}
            <strong>£50 non-refundable deposit per person</strong> at
            registration. Campers under 3 years old and day visitors are not
            required to pay a deposit at this stage.
          </p>
          <p className="mt-3 text-neutral-600 max-w-xl mx-auto text-sm">
            Pick a 1st and 2nd accommodation preference for each full-week
            camper. The White Team will allocate rooms after registration
            closes. Your accommodation choice will be secured once the full
            payment has been made.
          </p>
        </div>

        {/* Static site capacity overview. Display-only, no live allocation —
            just so families have a sense of what's on site before picking a
            preference. Numbers supplied by the White Team. */}
        <div className="bg-white border border-neutral-300 rounded-xl p-6 mb-8">
          <p className="text-xs font-bold uppercase tracking-widest text-neutral-500 mb-3">
            What&apos;s on site
          </p>
          <p className="text-sm text-neutral-600 mb-4">
            Approximate capacity across the camp. The White Team allocates
            rooms after registrations close — this is just for context.
          </p>
          <ul className="grid grid-cols-1 sm:grid-cols-2 gap-y-1.5 gap-x-6 text-sm text-neutral-800">
            <li>• 3 × Lodges — sleeps 8 each</li>
            <li>• 8 × Cabins — sleeps 2 each</li>
            <li>• 1 × Caravan — sleeps 4</li>
            <li>• 3 × Caravans — sleeps 4 each </li>
            <li>• 4 × Caravans — sleeps 6 each</li>
            <li>• 10 × Pods — sleeps 2 each</li>
            <li>• Tents — unlimited</li>
          </ul>
        </div>

        {topError && (
          <div className="mb-6 p-4 bg-red-50 border border-red-300 text-red-800 text-sm rounded">
            {topError}
          </div>
        )}

        <form onSubmit={onSubmit} className="flex flex-col gap-6">
          {state.campers.map((camper, i) => (
            <div key={i} className="flex flex-col gap-5">
              <CamperFieldset
                index={i}
                value={camper}
                isFirst={i === 0}
                accommodations={accommodations}
                shirtSizes={shirtSizes}
                fieldErrors={fieldErrors}
                onChange={(patch) => updateCamper(i, patch)}
                onRemove={i === 0 ? undefined : () => removeCamper(i)}
              />

              {i === 0 && (
                <fieldset className="bg-white border border-neutral-300 p-6 sm:p-8 rounded-xl shadow-sm flex flex-col gap-5">
                  <legend className="px-2 text-xs font-bold uppercase tracking-widest text-primary">
                    Contact details
                  </legend>
                  <p className="text-xs text-neutral-500 -mt-2">
                    We&apos;ll use these to send you payment confirmation from
                    Stripe.
                  </p>
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
                    <div className="flex flex-col gap-2">
                      <label htmlFor="contact-email" className={labelCls}>
                        Email <span className="text-primary">*</span>
                      </label>
                      <input
                        id="contact-email"
                        type="email"
                        value={state.contact.email}
                        required
                        onChange={(e) =>
                          updateContact({ email: e.target.value })
                        }
                        className={inputCls(!!fieldErrors["contact.email"])}
                      />
                      {fieldErrors["contact.email"] && (
                        <p className="text-xs text-red-600">
                          {fieldErrors["contact.email"]}
                        </p>
                      )}
                    </div>
                    <div className="flex flex-col gap-2">
                      <label htmlFor="contact-phone" className={labelCls}>
                        Phone <span className="text-primary">*</span>
                      </label>
                      <input
                        id="contact-phone"
                        type="tel"
                        value={state.contact.phone}
                        required
                        onChange={(e) =>
                          updateContact({ phone: e.target.value })
                        }
                        className={inputCls(!!fieldErrors["contact.phone"])}
                      />
                      {fieldErrors["contact.phone"] && (
                        <p className="text-xs text-red-600">
                          {fieldErrors["contact.phone"]}
                        </p>
                      )}
                    </div>
                  </div>
                </fieldset>
              )}
            </div>
          ))}

          {/* Add-another */}
          <div>
            <button
              type="button"
              onClick={addCamper}
              className="inline-flex items-center gap-2 px-6 py-3 bg-white border border-dashed border-primary text-primary text-xs uppercase tracking-widest font-bold hover:bg-primary hover:text-white transition-colors"
            >
              + Add another family or cell member
            </button>
          </div>

          {anyMinor && (
            <div className="p-4 bg-primary/10 border border-primary/30 text-sm text-neutral-700 rounded">
              <strong>Parental Consent Form:</strong> at least one camper is
              under 18. After you submit you&apos;ll be able to download the
              form — print it, sign it in ink, and hand the completed copy to
              Bro Ash before camp.
            </div>
          )}

          {/* Total + Submit */}
          <div className="bg-white border border-neutral-300 p-6 rounded-xl flex flex-col gap-4">
            <div className="flex flex-col sm:flex-row gap-4 sm:items-end sm:justify-between">
              <div>
                <p className="text-xs font-bold uppercase tracking-widest text-neutral-500">
                  Non-refundable deposit
                </p>
                <p className="text-3xl font-extrabold text-neutral-900">
                  {formatPence(totalPence, currency)}
                </p>
              </div>
              <button
                type="submit"
                disabled={submitting}
                className="px-10 py-4 bg-primary text-white font-bold uppercase tracking-widest text-sm hover:bg-primary-dark transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
              >
                {submitting
                  ? "Submitting…"
                  : totalPence === 0
                    ? "Submit registration"
                    : "Continue to payment"}
              </button>
            </div>
            {/* Breakdown */}
            {(payingCamperCount > 0 ||
              freeFullWeekCount > 0 ||
              dayPassCamperCount > 0) && (
              <ul className="text-xs text-neutral-600 border-t border-neutral-200 pt-3 flex flex-col gap-1">
                {payingCamperCount > 0 && (
                  <li>
                    {payingCamperCount} × full-week deposit (
                    {formatPence(depositPence, currency)} each) ={" "}
                    {formatPence(
                      depositPence * payingCamperCount,
                      currency,
                    )}
                  </li>
                )}
                {freeFullWeekCount > 0 && (
                  <li>
                    {freeFullWeekCount} × full-week camper under{" "}
                    {MIN_DEPOSIT_AGE} — no deposit required
                  </li>
                )}
                {dayPassCamperCount > 0 && (
                  <li>
                    {dayPassCamperCount} × day-pass camper — no deposit
                    required
                  </li>
                )}
              </ul>
            )}
            {totalPence === 0 &&
              payingCamperCount === 0 &&
              (freeFullWeekCount > 0 || dayPassCamperCount > 0) && (
                <p className="text-xs text-neutral-600 border-t border-neutral-200 pt-3">
                  No deposit is owed for this registration — you&apos;ll get
                  the confirmation email immediately after submitting.
                </p>
              )}
          </div>
        </form>
      </div>
    </section>
  );
}

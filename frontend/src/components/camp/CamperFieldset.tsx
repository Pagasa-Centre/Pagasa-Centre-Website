"use client";

import type { Accommodation, DayCode, ShirtSize } from "@/lib/api";
import {
  ACCOMMODATION_CHILD_CODE,
  ACCOMMODATION_TENT_CODE,
  type CamperState,
  DAY_PASS_DAYS,
  isMinor,
  MAX_CHILD_ACCOMMODATION_AGE,
  SHIRT_SIZE_NOT_APPLICABLE,
} from "@/lib/camp";
import AccommodationPicker, {
  type UnavailableCode,
} from "./AccommodationPicker";
import ShirtSizeSelect from "./ShirtSizeSelect";

type Props = {
  index: number;
  value: CamperState;
  isFirst: boolean;
  accommodations: Accommodation[];
  shirtSizes: ShirtSize[];
  fieldErrors: Record<string, string>;
  onChange: (patch: Partial<CamperState>) => void;
  onRemove?: () => void;
};

const labelCls =
  "text-xs font-bold uppercase tracking-widest text-neutral-700";
const baseInput =
  "px-4 py-3 bg-white border text-neutral-900 placeholder-neutral-500 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary";

function inputCls(hasError: boolean) {
  return `${baseInput} ${hasError ? "border-red-500" : "border-neutral-300"}`;
}

function fieldError(
  errors: Record<string, string>,
  prefix: string,
  suffix: string,
): string | undefined {
  return errors[`${prefix}.${suffix}`];
}

export default function CamperFieldset({
  index,
  value,
  isFirst,
  accommodations,
  shirtSizes,
  fieldErrors,
  onChange,
  onRemove,
}: Props) {
  const prefix = `campers[${index}]`;
  const idFor = (k: string) => `camper-${index}-${k}`;
  const err = (k: string) => fieldError(fieldErrors, prefix, k);
  const attErr = (k: string) =>
    fieldError(fieldErrors, `${prefix}.attendance`, k);
  const ageN = parseInt(value.age, 10);
  const showMinorBanner = Number.isFinite(ageN) && isMinor(ageN);
  // Child-with-parent accommodation is only valid for under-13s. We make the
  // option un-selectable in the picker as soon as the age has been typed,
  // but if it's been *pre*-selected and the age is bumped past the limit,
  // wipe the stale choice so we don't silently submit something invalid.
  const childAccommodationLocked =
    Number.isFinite(ageN) && ageN > MAX_CHILD_ACCOMMODATION_AGE;
  const childUnavailable: UnavailableCode[] = childAccommodationLocked
    ? [
        {
          code: ACCOMMODATION_CHILD_CODE,
          reason: `Ages 1\u2013${MAX_CHILD_ACCOMMODATION_AGE} only`,
        },
      ]
    : [];
  // Types the White Team has disabled for registration render as greyed,
  // non-selectable tiles rather than being hidden.
  const adminUnavailable: UnavailableCode[] = accommodations
    .filter((a) => !a.available_for_registration)
    .map((a) => ({ code: a.code, reason: "Not available" }));

  return (
    <fieldset className="bg-white border border-neutral-300 p-6 sm:p-8 rounded-xl shadow-sm flex flex-col gap-5">
      <legend className="px-2 text-xs font-bold uppercase tracking-widest text-primary">
        {isFirst
          ? "Section 1 — Main Contact"
          : `Additional Camper ${index}`}
      </legend>

      {/* Name */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
        <div className="flex flex-col gap-2">
          <label htmlFor={idFor("first")} className={labelCls}>
            First name(s) <span className="text-primary">*</span>
          </label>
          <input
            id={idFor("first")}
            type="text"
            value={value.first_name}
            required
            onChange={(e) => onChange({ first_name: e.target.value })}
            className={inputCls(!!err("first_name"))}
          />
          {err("first_name") && (
            <p className="text-xs text-red-600">{err("first_name")}</p>
          )}
        </div>
        <div className="flex flex-col gap-2">
          <label htmlFor={idFor("last")} className={labelCls}>
            Last name <span className="text-primary">*</span>
          </label>
          <input
            id={idFor("last")}
            type="text"
            value={value.last_name}
            required
            onChange={(e) => onChange({ last_name: e.target.value })}
            className={inputCls(!!err("last_name"))}
          />
          {err("last_name") && (
            <p className="text-xs text-red-600">{err("last_name")}</p>
          )}
        </div>
      </div>

      {/* Gender + Age */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
        <div className="flex flex-col gap-2">
          <span className={labelCls}>
            Gender <span className="text-primary">*</span>
          </span>
          <div className="flex gap-4 pt-1">
            {(["male", "female"] as const).map((g) => (
              <label
                key={g}
                className="flex items-center gap-2 text-sm text-neutral-800"
              >
                <input
                  type="radio"
                  name={`${prefix}.gender`}
                  value={g}
                  checked={value.gender === g}
                  required
                  onChange={() => onChange({ gender: g })}
                  className="text-primary focus:ring-primary"
                />
                <span className="capitalize">{g}</span>
              </label>
            ))}
          </div>
          {err("gender") && (
            <p className="text-xs text-red-600">{err("gender")}</p>
          )}
        </div>
        <div className="flex flex-col gap-2">
          <label htmlFor={idFor("age")} className={labelCls}>
            Age <span className="text-primary">*</span>
          </label>
          <input
            id={idFor("age")}
            type="number"
            min={1}
            max={119}
            value={value.age}
            required
            onChange={(e) => {
              const next = e.target.value;
              const parsed = parseInt(next, 10);
              const patch: Partial<CamperState> = { age: next };
              // If the new age makes the child accommodation invalid, wipe
              // any previously-selected child choice. Keeps state and the
              // backend in sync.
              if (
                Number.isFinite(parsed) &&
                parsed > MAX_CHILD_ACCOMMODATION_AGE
              ) {
                if (
                  value.accommodation_first_choice === ACCOMMODATION_CHILD_CODE
                ) {
                  patch.accommodation_first_choice = "";
                }
                if (
                  value.accommodation_second_choice === ACCOMMODATION_CHILD_CODE
                ) {
                  patch.accommodation_second_choice = "";
                }
              }
              onChange(patch);
            }}
            className={inputCls(!!err("age"))}
          />
          {err("age") && <p className="text-xs text-red-600">{err("age")}</p>}
          {showMinorBanner && (
            <div className="mt-1 p-3 bg-primary/10 border border-primary/30 text-xs text-neutral-700">
              <strong>Under 18:</strong> All campers under 18 attending without
              their parent/guardian must complete a Parental Consent Form. It
              will be made available for download after you submit.
            </div>
          )}
        </div>
      </div>

      {/* Cell leader name + checkbox */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-5 items-end">
        <div className="flex flex-col gap-2">
          <label htmlFor={idFor("cell")} className={labelCls}>
            Cell leader name <span className="text-primary">*</span>
          </label>
          <input
            id={idFor("cell")}
            type="text"
            value={value.cell_leader_name}
            required
            onChange={(e) =>
              onChange({ cell_leader_name: e.target.value })
            }
            className={inputCls(!!err("cell_leader_name"))}
          />
          {err("cell_leader_name") && (
            <p className="text-xs text-red-600">{err("cell_leader_name")}</p>
          )}
        </div>
        <label className="flex items-center gap-3 text-sm text-neutral-800 pb-3">
          <input
            type="checkbox"
            checked={value.is_cell_leader}
            onChange={(e) =>
              onChange({ is_cell_leader: e.target.checked })
            }
            className="w-4 h-4 text-primary focus:ring-primary"
          />
          {isFirst ? "I am a cell leader" : "They are a cell leader"}
        </label>
      </div>

      {/* Attendance type */}
      <div className="flex flex-col gap-2 pt-2 border-t border-neutral-200">
        <span className={labelCls}>
          Attendance <span className="text-primary">*</span>
        </span>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-2">
          {(
            [
              {
                code: "full_week",
                label: "Full week (10–14 Aug)",
              },
              {
                code: "day_pass",
                label: "Day pass",
              },
            ] as const
          ).map((opt) => (
            <label
              key={opt.code}
              className={`flex items-center gap-3 p-4 border cursor-pointer transition-colors ${
                value.attendance_type === opt.code
                  ? "bg-primary/5 border-primary"
                  : "bg-white border-neutral-300 hover:border-primary/60"
              }`}
            >
              <input
                type="radio"
                name={`${prefix}.attendance_type`}
                value={opt.code}
                checked={value.attendance_type === opt.code}
                required
                onChange={() =>
                  onChange({ attendance_type: opt.code })
                }
                className="text-primary focus:ring-primary"
              />
              <span className="text-sm font-semibold text-neutral-900">
                {opt.label}
              </span>
            </label>
          ))}
        </div>
        {attErr("type") && (
          <p className="text-xs text-red-600">{attErr("type")}</p>
        )}
      </div>

      {/* Full-week branch */}
      {value.attendance_type === "full_week" && (
        <div className="flex flex-col gap-5 pt-2">
          <ShirtSizeSelect
            id={idFor("shirt")}
            name={`${prefix}.shirt_size`}
            value={value.shirt_size}
            sizes={shirtSizes}
            required
            error={attErr("shirt_size")}
            onChange={(s) => onChange({ shirt_size: s })}
          />
          <div className="flex flex-col gap-2">
            <label htmlFor={idFor("diet")} className={labelCls}>
              Allergies / dietary requirements
            </label>
            <textarea
              id={idFor("diet")}
              rows={2}
              value={value.dietary_requirements}
              placeholder="Type N/A if none"
              onChange={(e) =>
                onChange({ dietary_requirements: e.target.value })
              }
              className={`${inputCls(false)} resize-y`}
            />
          </div>
          <div className="flex flex-col gap-2">
            <span className={labelCls}>
              Transportation <span className="text-primary">*</span>
            </span>
            <div className="flex flex-col gap-2 mt-1">
              {(
                [
                  { v: false, label: "I have made my own arrangements" },
                  { v: true, label: "I will need a spot on the coach" },
                ] as const
              ).map((opt) => (
                <label
                  key={String(opt.v)}
                  className="flex items-center gap-3 text-sm text-neutral-800"
                >
                  <input
                    type="radio"
                    name={`${prefix}.needs_coach`}
                    checked={value.needs_coach === opt.v}
                    required
                    onChange={() => onChange({ needs_coach: opt.v })}
                    className="text-primary focus:ring-primary"
                  />
                  {opt.label}
                </label>
              ))}
            </div>
          </div>
          <div className="flex flex-col gap-2 pt-2">
            <p className="text-sm text-neutral-700 leading-relaxed">
              {value.accommodation_first_choice === ACCOMMODATION_TENT_CODE ? (
                <>
                  You&apos;ve chosen a <strong>tent</strong> — no 2nd choice
                  needed, as there&apos;s always room to pitch your own.
                </>
              ) : (
                <>
                  Pick a <strong>1st</strong> and <strong>2nd</strong>{" "}
                  accommodation preference. The White Team will place every
                  camper after registrations close — priority goes to women,
                  elderly, and families, so we can&apos;t guarantee your first
                  choice.
                </>
              )}
            </p>
          </div>
          <AccommodationPicker
            name={`${prefix}.accommodation_first_choice`}
            label="1st choice accommodation"
            value={value.accommodation_first_choice}
            accommodations={accommodations}
            unavailable={[...childUnavailable, ...adminUnavailable]}
            error={attErr("accommodation_first_choice")}
            onChange={(code) => {
              const patch: Partial<CamperState> = {
                accommodation_first_choice: code,
              };
              if (
                code === ACCOMMODATION_CHILD_CODE ||
                code === ACCOMMODATION_TENT_CODE
              ) {
                // Child-with-parent and tent have no meaningful 2nd choice;
                // wipe any stale value so we don't accidentally submit it.
                patch.accommodation_second_choice = "";
              } else if (value.accommodation_second_choice === code) {
                // Picking the same code as 2nd; clear 2nd to force a re-pick.
                patch.accommodation_second_choice = "";
              }
              onChange(patch);
            }}
          />
          {value.accommodation_first_choice !== ACCOMMODATION_CHILD_CODE &&
            value.accommodation_first_choice !== ACCOMMODATION_TENT_CODE && (
            <AccommodationPicker
              name={`${prefix}.accommodation_second_choice`}
              label="2nd choice accommodation"
              value={value.accommodation_second_choice}
              accommodations={accommodations}
              unavailable={[
                ...(value.accommodation_first_choice
                  ? [
                      {
                        code: value.accommodation_first_choice,
                        reason: "Already 1st choice",
                      },
                    ]
                  : []),
                ...childUnavailable,
                ...adminUnavailable,
              ]}
              error={attErr("accommodation_second_choice")}
              onChange={(code) =>
                onChange({ accommodation_second_choice: code })
              }
            />
          )}
          <div className="flex flex-col gap-2">
            <label htmlFor={idFor("roommate")} className={labelCls}>
              Roommate requests
            </label>
            <textarea
              id={idFor("roommate")}
              rows={3}
              maxLength={500}
              value={value.roommate_requests}
              placeholder="e.g. sharing with my partner Jane Doe, or with the cell group of Pastor Bob. Leave blank if no preference."
              onChange={(e) =>
                onChange({ roommate_requests: e.target.value })
              }
              className={`${inputCls(!!attErr("roommate_requests"))} resize-y`}
            />
            <p className="text-xs text-neutral-500">
              The White Team will try their best to keep couples and friend
              groups in the same accommodation, but can&apos;t guarantee it.
            </p>
            {attErr("roommate_requests") && (
              <p className="text-xs text-red-600">
                {attErr("roommate_requests")}
              </p>
            )}
          </div>
        </div>
      )}

      {/* Day-pass branch */}
      {value.attendance_type === "day_pass" && (
        <div className="flex flex-col gap-5 pt-2">
          <div className="flex flex-col gap-2">
            <span className={labelCls}>
              Days attending <span className="text-primary">*</span>
            </span>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 mt-1">
              {DAY_PASS_DAYS.map((d) => {
                const checked = value.day_pass_days.includes(d.code);
                return (
                  <label
                    key={d.code}
                    className={`flex items-center gap-3 p-3 border cursor-pointer text-sm text-neutral-800 transition-colors ${
                      checked
                        ? "bg-primary/5 border-primary"
                        : "bg-white border-neutral-300 hover:border-primary/60"
                    }`}
                  >
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={(e) => {
                        const next: DayCode[] = e.target.checked
                          ? [...value.day_pass_days, d.code]
                          : value.day_pass_days.filter((x) => x !== d.code);
                        onChange({ day_pass_days: next });
                      }}
                      className="w-4 h-4 text-primary focus:ring-primary"
                    />
                    {d.label}
                  </label>
                );
              })}
            </div>
            {attErr("days") && (
              <p className="text-xs text-red-600">{attErr("days")}</p>
            )}
          </div>

          <div className="flex flex-col gap-2">
            <span className={labelCls}>
              T-shirt <span className="text-primary">*</span>
            </span>
            <div className="flex flex-col gap-2 mt-1">
              {(
                [
                  {
                    v: "team_activities",
                    label:
                      "I will be participating in team activities (T-shirt required)",
                  },
                  {
                    v: "tshirt_only",
                    label: "I will be purchasing an official camp T-shirt",
                  },
                  { v: "none", label: "I will not be purchasing a T-shirt" },
                ] as const
              ).map((opt) => (
                <label
                  key={opt.v}
                  className="flex items-start gap-3 text-sm text-neutral-800"
                >
                  <input
                    type="radio"
                    name={`${prefix}.tshirt_option`}
                    value={opt.v}
                    checked={value.tshirt_option === opt.v}
                    required
                    onChange={() => {
                      // If switching to "none", set shirt_size to "n/a";
                      // otherwise clear it so the user picks a size.
                      onChange({
                        tshirt_option: opt.v,
                        shirt_size:
                          opt.v === "none" ? SHIRT_SIZE_NOT_APPLICABLE : "",
                      });
                    }}
                    className="text-primary focus:ring-primary mt-0.5"
                  />
                  <span>{opt.label}</span>
                </label>
              ))}
            </div>
            {attErr("tshirt_option") && (
              <p className="text-xs text-red-600">{attErr("tshirt_option")}</p>
            )}
          </div>

          <ShirtSizeSelect
            id={idFor("shirt")}
            name={`${prefix}.shirt_size`}
            value={
              value.tshirt_option === "none" ? "" : value.shirt_size
            }
            sizes={shirtSizes}
            required={value.tshirt_option !== "none" && value.tshirt_option !== ""}
            disabled={value.tshirt_option === "none"}
            error={attErr("shirt_size")}
            onChange={(s) => onChange({ shirt_size: s })}
          />

          <div className="flex flex-col gap-2">
            <span className={labelCls}>
              Will you require catering? <span className="text-primary">*</span>
            </span>
            <div className="flex gap-4 mt-1">
              {(
                [
                  { v: true, label: "Yes" },
                  { v: false, label: "No" },
                ] as const
              ).map((opt) => (
                <label
                  key={String(opt.v)}
                  className="flex items-center gap-2 text-sm text-neutral-800"
                >
                  <input
                    type="radio"
                    name={`${prefix}.needs_catering`}
                    checked={value.needs_catering === opt.v}
                    required
                    onChange={() => onChange({ needs_catering: opt.v })}
                    className="text-primary focus:ring-primary"
                  />
                  {opt.label}
                </label>
              ))}
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <label htmlFor={idFor("diet")} className={labelCls}>
              Allergies / dietary requirements
            </label>
            <textarea
              id={idFor("diet")}
              rows={2}
              value={value.dietary_requirements}
              placeholder="Type N/A if none"
              onChange={(e) =>
                onChange({ dietary_requirements: e.target.value })
              }
              className={`${inputCls(false)} resize-y`}
            />
          </div>
        </div>
      )}

      {onRemove && (
        <div className="pt-2 border-t border-neutral-200 flex justify-end">
          <button
            type="button"
            onClick={onRemove}
            className="text-xs uppercase tracking-widest font-bold text-red-600 hover:text-red-800"
          >
            Remove camper
          </button>
        </div>
      )}
    </fieldset>
  );
}

"use client";

import type { Accommodation } from "@/lib/api";

/**
 * UnavailableCode marks an accommodation code as not selectable, with a
 * short reason rendered as a badge on the tile. Used for both the
 * "already picked as 1st choice" case and the age-gate on child
 * accommodation.
 */
export type UnavailableCode = {
  code: string;
  reason: string;
};

type Props = {
  name: string;
  value: string;
  label: string;
  accommodations: Accommodation[];
  /**
   * Codes that should be rendered disabled, with the reason shown as a
   * badge on the tile.
   */
  unavailable?: UnavailableCode[];
  error?: string;
  onChange: (code: string) => void;
};

export default function AccommodationPicker({
  name,
  value,
  label,
  accommodations,
  unavailable = [],
  error,
  onChange,
}: Props) {
  const unavailableMap = new Map(unavailable.map((u) => [u.code, u.reason]));
  return (
    <div className="flex flex-col gap-2">
      <p className="text-xs font-bold uppercase tracking-widest text-neutral-700">
        {label} <span className="text-primary">*</span>
      </p>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-2">
        {accommodations.map((a) => {
          const selected = value === a.code;
          const reason = unavailableMap.get(a.code);
          const disabled = reason !== undefined;
          return (
            <label
              key={a.code}
              className={`relative flex flex-col gap-1 p-4 border transition-colors ${
                disabled
                  ? "bg-neutral-100 border-neutral-300 cursor-not-allowed opacity-60"
                  : selected
                    ? "bg-primary/5 border-primary cursor-pointer"
                    : "bg-white border-neutral-300 hover:border-primary/60 cursor-pointer"
              }`}
            >
              <input
                type="radio"
                name={name}
                value={a.code}
                checked={selected}
                disabled={disabled}
                onChange={() => onChange(a.code)}
                className="sr-only"
              />
              <div className="flex items-start justify-between gap-2">
                <span className="font-semibold text-neutral-900 text-sm">
                  {a.display_name}
                </span>
                {reason && (
                  <span className="text-[10px] font-bold uppercase tracking-wider px-2 py-0.5 bg-neutral-300 text-neutral-700 whitespace-nowrap">
                    {reason}
                  </span>
                )}
              </div>
              {a.notes && (
                <p className="text-xs text-neutral-500 mt-1">{a.notes}</p>
              )}
            </label>
          );
        })}
      </div>
      {error && <p className="text-xs text-red-600">{error}</p>}
    </div>
  );
}

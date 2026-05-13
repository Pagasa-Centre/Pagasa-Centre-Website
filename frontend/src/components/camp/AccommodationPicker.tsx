"use client";

import type { Accommodation } from "@/lib/api";

type Props = {
  name: string;
  value: string;
  accommodations: Accommodation[];
  error?: string;
  onChange: (code: string) => void;
};

function remainingLabel(a: Accommodation): string {
  if (a.remaining === null) return "No limit";
  if (a.remaining <= 0) return "Full";
  return `${a.remaining} slot${a.remaining === 1 ? "" : "s"} left`;
}

export default function AccommodationPicker({
  name,
  value,
  accommodations,
  error,
  onChange,
}: Props) {
  return (
    <div className="flex flex-col gap-2">
      <p className="text-xs font-bold uppercase tracking-widest text-neutral-700">
        Accommodation <span className="text-primary">*</span>
      </p>
      <p className="text-xs text-neutral-500">
        First come, first served. Numbers update each time the page loads.
      </p>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-2">
        {accommodations.map((a) => {
          const soldOut = a.remaining !== null && a.remaining <= 0;
          const selected = value === a.code;
          return (
            <label
              key={a.code}
              className={`relative flex flex-col gap-1 p-4 border cursor-pointer transition-colors ${
                soldOut
                  ? "bg-neutral-100 border-neutral-300 cursor-not-allowed opacity-60"
                  : selected
                    ? "bg-primary/5 border-primary"
                    : "bg-white border-neutral-300 hover:border-primary/60"
              }`}
            >
              <input
                type="radio"
                name={name}
                value={a.code}
                checked={selected}
                disabled={soldOut}
                onChange={() => onChange(a.code)}
                className="sr-only"
              />
              <div className="flex items-center justify-between gap-2">
                <span className="font-semibold text-neutral-900 text-sm">
                  {a.display_name}
                </span>
                <span
                  className={`text-xs font-bold uppercase tracking-wider px-2 py-0.5 ${
                    soldOut
                      ? "bg-neutral-300 text-neutral-600"
                      : a.remaining === null
                        ? "bg-neutral-800 text-white"
                        : "bg-primary text-white"
                  }`}
                >
                  {remainingLabel(a)}
                </span>
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

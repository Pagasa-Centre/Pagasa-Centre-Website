"use client";

import type { ShirtSize } from "@/lib/api";

type Props = {
  id: string;
  name: string;
  value: string;
  sizes: ShirtSize[];
  required: boolean;
  disabled?: boolean;
  error?: string;
  onChange: (next: string) => void;
};

export default function ShirtSizeSelect({
  id,
  name,
  value,
  sizes,
  required,
  disabled,
  error,
  onChange,
}: Props) {
  const adult = sizes.filter((s) => s.category === "adult");
  const child = sizes.filter((s) => s.category === "child");

  return (
    <div className="flex flex-col gap-2">
      <label
        htmlFor={id}
        className="text-xs font-bold uppercase tracking-widest text-neutral-700"
      >
        Shirt size {required && <span className="text-primary">*</span>}
      </label>
      <select
        id={id}
        name={name}
        value={value}
        required={required && !disabled}
        disabled={disabled}
        onChange={(e) => onChange(e.target.value)}
        className={`px-4 py-3 bg-white border ${
          error ? "border-red-500" : "border-neutral-300"
        } text-neutral-900 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary disabled:bg-neutral-100 disabled:text-neutral-400`}
      >
        <option value="">{disabled ? "Not applicable" : "Select a size…"}</option>
        <optgroup label="Adult">
          {adult.map((s) => (
            <option key={s.code} value={s.code}>
              {s.display_name}
            </option>
          ))}
        </optgroup>
        <optgroup label="Child">
          {child.map((s) => (
            <option key={s.code} value={s.code}>
              {s.display_name}
            </option>
          ))}
        </optgroup>
      </select>
      {!disabled && (
        <details className="text-xs text-neutral-500">
          <summary className="cursor-pointer select-none font-semibold text-neutral-600">
            Adult size guide (chest)
          </summary>
          <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1">
            <span>S 34/36&quot;</span>
            <span>M 38/40&quot;</span>
            <span>L 42/44&quot;</span>
            <span>XL 46/48&quot;</span>
            <span>2XL 50/52&quot;</span>
            <span>3XL 54/56&quot;</span>
          </div>
        </details>
      )}
      {error && <p className="text-xs text-red-600">{error}</p>}
    </div>
  );
}

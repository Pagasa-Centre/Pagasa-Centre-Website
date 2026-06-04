"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  adminApi,
  type AdminAccommodation,
  type AdminGroup,
  type AdminCamper,
  type AllocateCamper,
  AdminApiError,
} from "@/lib/admin-api";
import {
  ACCOMMODATION_CHILD_CODE,
  MAX_CHILD_ACCOMMODATION_AGE,
  MIN_DEPOSIT_AGE,
  formatPence,
} from "@/lib/camp";

type AllocState = Record<string, string>; // camperId -> accommodation code

// The single source of truth for "what should the White Team do next" with a
// group. Everything (tabs, tiles, badges, action buttons) keys off this.
type Category =
  | "to_allocate"
  | "to_invoice"
  | "awaiting"
  | "paid"
  | "daypass"
  | "unpaid";

type TabKey = "to_allocate" | "to_invoice" | "awaiting" | "paid" | "all";

const TABS: { key: TabKey; label: string }[] = [
  { key: "to_allocate", label: "Needs accommodation" },
  { key: "to_invoice", label: "Ready to invoice" },
  { key: "awaiting", label: "Awaiting payment" },
  { key: "paid", label: "Paid" },
  { key: "all", label: "Everyone" },
];

function fullWeekCampers(g: AdminGroup): AdminCamper[] {
  return g.campers.filter((c) => c.attendance_type === "full_week");
}

function isOverdue(g: AdminGroup): boolean {
  return (
    g.billing_status === "invoiced" &&
    !!g.invoice_due_at &&
    new Date(g.invoice_due_at) < new Date()
  );
}

function categorize(g: AdminGroup): Category {
  if (g.payment_status !== "paid") return "unpaid";
  if (fullWeekCampers(g).length === 0) return "daypass";
  switch (g.billing_status) {
    case "invoiced":
      return "awaiting";
    case "balance_paid":
      return "paid";
    case "allocated":
      return "to_invoice";
    default: // "none" or "released" → still needs (re)allocation
      return "to_allocate";
  }
}

function statusBadge(g: AdminGroup): { label: string; className: string } {
  if (isOverdue(g)) {
    return {
      label: "Payment overdue",
      className: "bg-red-100 text-red-800 border-red-200",
    };
  }
  switch (categorize(g)) {
    case "to_allocate":
      return g.billing_status === "released"
        ? {
            label: "Released — re-allocate",
            className: "bg-neutral-200 text-neutral-700 border-neutral-300",
          }
        : {
            label: "Needs accommodation",
            className: "bg-amber-100 text-amber-800 border-amber-200",
          };
    case "to_invoice":
      return {
        label: "Ready to invoice",
        className: "bg-blue-100 text-blue-800 border-blue-200",
      };
    case "awaiting":
      return {
        label: "Awaiting payment",
        className: "bg-violet-100 text-violet-800 border-violet-200",
      };
    case "paid":
      return {
        label: "Paid in full",
        className: "bg-green-100 text-green-800 border-green-200",
      };
    case "daypass":
      return {
        label: "Day pass only",
        className: "bg-neutral-200 text-neutral-600 border-neutral-300",
      };
    default:
      return {
        label: "Deposit unpaid",
        className: "bg-neutral-200 text-neutral-600 border-neutral-300",
      };
  }
}

function formatDate(iso: string | null | undefined): string {
  if (!iso) return "—";
  try {
    return new Date(iso).toLocaleDateString("en-GB", {
      day: "numeric",
      month: "short",
      year: "numeric",
    });
  } catch {
    return iso;
  }
}

export default function AdminDashboard() {
  const router = useRouter();
  const [groups, setGroups] = useState<AdminGroup[]>([]);
  const [accommodations, setAccommodations] = useState<AdminAccommodation[]>([]);
  const [alloc, setAlloc] = useState<Record<string, AllocState>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [tab, setTab] = useState<TabKey>("to_allocate");
  const [search, setSearch] = useState("");

  const accName = useCallback(
    (code: string | null | undefined): string => {
      if (!code) return "—";
      return accommodations.find((a) => a.code === code)?.display_name ?? code;
    },
    [accommodations],
  );

  const load = useCallback(async () => {
    setError(null);
    try {
      const [reg, acc] = await Promise.all([
        adminApi.listRegistrations(),
        adminApi.accommodations(),
      ]);
      setGroups(reg.groups);
      setAccommodations(acc.accommodations);
      const next: Record<string, AllocState> = {};
      for (const g of reg.groups) {
        const m: AllocState = {};
        for (const c of g.campers) {
          if (c.allocated_accommodation_code) {
            m[c.id] = c.allocated_accommodation_code;
          } else if (
            c.attendance_type === "full_week" &&
            c.accommodation_first_choice
          ) {
            m[c.id] = c.accommodation_first_choice;
          }
        }
        next[g.id] = m;
      }
      setAlloc(next);
    } catch (err) {
      if (err instanceof AdminApiError && err.status === 401) {
        router.replace("/admin/login");
        return;
      }
      setError("Could not load registrations. Please refresh the page.");
    } finally {
      setLoading(false);
    }
  }, [router]);

  useEffect(() => {
    setLoading(true);
    adminApi
      .checkSession()
      .then(() => load())
      .catch(() => router.replace("/admin/login"));
  }, [load, router]);

  function setCamperAlloc(groupId: string, camperId: string, code: string) {
    setAlloc((prev) => ({
      ...prev,
      [groupId]: { ...prev[groupId], [camperId]: code },
    }));
  }

  function allFullWeekAllocated(g: AdminGroup): boolean {
    const fw = fullWeekCampers(g);
    if (fw.length === 0) return false;
    const state = alloc[g.id] ?? {};
    return fw.every((c) => !!state[c.id]);
  }

  async function saveAllocation(g: AdminGroup) {
    setBusy(`alloc-${g.id}`);
    setError(null);
    setNotice(null);
    try {
      const campers: AllocateCamper[] = fullWeekCampers(g).map((c) => ({
        camper_id: c.id,
        allocated_accommodation_code: alloc[g.id]?.[c.id] ?? "",
      }));
      await adminApi.saveAllocation(g.id, campers);
      setNotice(`Accommodation saved for ${g.contact_first_name}'s group.`);
      await load();
    } catch (err) {
      setError(
        err instanceof AdminApiError ? err.detail.message : "Save failed.",
      );
    } finally {
      setBusy(null);
    }
  }

  async function sendInvoice(g: AdminGroup) {
    setBusy(`inv-${g.id}`);
    setError(null);
    setNotice(null);
    try {
      await adminApi.sendInvoice(g.id);
      setNotice(`Invoice emailed to ${g.contact_email}.`);
      await load();
    } catch (err) {
      setError(
        err instanceof AdminApiError ? err.detail.message : "Invoice failed.",
      );
    } finally {
      setBusy(null);
    }
  }

  async function sendAllAllocated() {
    const ids = groups
      .filter((g) => categorize(g) === "to_invoice")
      .map((g) => g.id);
    if (ids.length === 0) return;
    if (
      !confirm(
        `Send balance invoices to ${ids.length} group(s) now? Each person will get an email from Stripe.`,
      )
    ) {
      return;
    }
    setBusy("bulk");
    setError(null);
    setNotice(null);
    try {
      const res = await adminApi.sendInvoiceBulk(ids);
      if (res && typeof res === "object" && "errors" in res) {
        const errs = (res as { errors: Record<string, string> }).errors;
        setError(
          `Sent, but ${Object.keys(errs).length} group(s) failed. Try those again individually.`,
        );
      } else {
        setNotice(`Sent ${ids.length} invoice(s).`);
      }
      await load();
    } catch (err) {
      setError(
        err instanceof AdminApiError ? err.detail.message : "Bulk send failed.",
      );
    } finally {
      setBusy(null);
    }
  }

  async function releaseGroup(g: AdminGroup) {
    if (
      !confirm(
        `Release the accommodation for ${g.contact_first_name} ${g.contact_last_name}?\n\nThis cancels their unpaid invoice and frees up their spot.`,
      )
    ) {
      return;
    }
    setBusy(`rel-${g.id}`);
    setError(null);
    setNotice(null);
    try {
      await adminApi.release(g.id);
      setNotice(`Released ${g.contact_first_name}'s accommodation.`);
      await load();
    } catch (err) {
      setError(
        err instanceof AdminApiError ? err.detail.message : "Release failed.",
      );
    } finally {
      setBusy(null);
    }
  }

  async function resendInvoice(g: AdminGroup) {
    setBusy(`res-${g.id}`);
    setError(null);
    setNotice(null);
    try {
      await adminApi.resendInvoice(g.id);
      setNotice(`Reminder re-sent to ${g.contact_email}.`);
    } catch (err) {
      setError(
        err instanceof AdminApiError ? err.detail.message : "Resend failed.",
      );
    } finally {
      setBusy(null);
    }
  }

  async function extendDue(g: AdminGroup) {
    const days = prompt(
      "Give them more time to pay — how many extra days from today?",
      "7",
    );
    if (!days) return;
    const n = parseInt(days, 10);
    if (!Number.isFinite(n) || n <= 0) return;
    const due = new Date();
    due.setDate(due.getDate() + n);
    setBusy(`ext-${g.id}`);
    setError(null);
    setNotice(null);
    try {
      await adminApi.extendDue(g.id, due.toISOString());
      setNotice(`New due date set to ${formatDate(due.toISOString())}.`);
      await load();
    } catch (err) {
      setError(
        err instanceof AdminApiError ? err.detail.message : "Extend failed.",
      );
    } finally {
      setBusy(null);
    }
  }

  async function runSweep() {
    if (
      !confirm(
        "Cancel all invoices that are past their due date and free up those spots?",
      )
    ) {
      return;
    }
    setBusy("sweep");
    setError(null);
    setNotice(null);
    try {
      const res = await adminApi.sweep();
      setNotice(`Released ${res.released} overdue group(s).`);
      await load();
    } catch (err) {
      setError(
        err instanceof AdminApiError ? err.detail.message : "Sweep failed.",
      );
    } finally {
      setBusy(null);
    }
  }

  async function logout() {
    await adminApi.logout();
    router.replace("/admin/login");
  }

  const counts = useMemo(() => {
    const c = {
      to_allocate: 0,
      to_invoice: 0,
      awaiting: 0,
      paid: 0,
      overdue: 0,
      all: groups.length,
    };
    for (const g of groups) {
      const cat = categorize(g);
      if (cat === "to_allocate") c.to_allocate++;
      else if (cat === "to_invoice") c.to_invoice++;
      else if (cat === "awaiting") {
        c.awaiting++;
        if (isOverdue(g)) c.overdue++;
      } else if (cat === "paid") c.paid++;
    }
    return c;
  }, [groups]);

  const visibleGroups = useMemo(() => {
    const q = search.trim().toLowerCase();
    return groups
      .filter((g) => {
        const cat = categorize(g);
        if (tab === "all") {
          // Everyone except not-yet-deposit-paid clutter.
          if (cat === "unpaid") return false;
        } else if (cat !== tab) {
          return false;
        }
        if (!q) return true;
        const hay =
          `${g.contact_first_name} ${g.contact_last_name} ${g.contact_email}`.toLowerCase();
        return hay.includes(q);
      })
      .sort((a, b) => {
        // Overdue first, then by surname.
        const ao = isOverdue(a) ? 0 : 1;
        const bo = isOverdue(b) ? 0 : 1;
        if (ao !== bo) return ao - bo;
        return a.contact_last_name.localeCompare(b.contact_last_name);
      });
  }, [groups, tab, search]);

  if (loading) {
    return (
      <div className="py-24 text-center text-neutral-500">
        Loading registrations…
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      {/* Title + sign out */}
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-extrabold text-neutral-800">
            Camp registrations
          </h1>
          <p className="text-sm text-neutral-600 mt-1">
            Allocate accommodation, then send each group their balance invoice.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => load()}
            className="px-3 py-2 text-sm font-semibold text-neutral-700 bg-white border border-neutral-300 rounded-lg hover:bg-neutral-100"
          >
            Refresh
          </button>
          <button
            type="button"
            onClick={logout}
            className="px-3 py-2 text-sm font-semibold text-neutral-700 bg-white border border-neutral-300 rounded-lg hover:bg-neutral-100"
          >
            Sign out
          </button>
        </div>
      </div>

      {/* How it works */}
      <div className="bg-white border border-neutral-300 rounded-xl p-4 sm:p-5">
        <p className="text-sm font-bold text-neutral-800 mb-2">How this works</p>
        <ol className="grid sm:grid-cols-3 gap-3 text-sm text-neutral-700">
          <li className="flex gap-2">
            <span className="flex-none w-6 h-6 rounded-full bg-amber-100 text-amber-800 font-bold text-xs flex items-center justify-center">
              1
            </span>
            <span>
              <strong>Allocate</strong> — choose where each person stays, then
              press <em>Save accommodation</em>.
            </span>
          </li>
          <li className="flex gap-2">
            <span className="flex-none w-6 h-6 rounded-full bg-blue-100 text-blue-800 font-bold text-xs flex items-center justify-center">
              2
            </span>
            <span>
              <strong>Invoice</strong> — press <em>Send balance invoice</em>.
              Stripe emails them a secure payment link.
            </span>
          </li>
          <li className="flex gap-2">
            <span className="flex-none w-6 h-6 rounded-full bg-green-100 text-green-800 font-bold text-xs flex items-center justify-center">
              3
            </span>
            <span>
              <strong>Done</strong> — paid groups show green. Unpaid invoices
              are released automatically after the due date.
            </span>
          </li>
        </ol>
      </div>

      {/* Stat tiles (clickable) */}
      <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
        <StatTile
          label="Needs accommodation"
          value={counts.to_allocate}
          active={tab === "to_allocate"}
          tone="amber"
          onClick={() => setTab("to_allocate")}
        />
        <StatTile
          label="Ready to invoice"
          value={counts.to_invoice}
          active={tab === "to_invoice"}
          tone="blue"
          onClick={() => setTab("to_invoice")}
        />
        <StatTile
          label="Awaiting payment"
          value={counts.awaiting}
          active={tab === "awaiting"}
          tone="violet"
          onClick={() => setTab("awaiting")}
        />
        <StatTile
          label="Overdue"
          value={counts.overdue}
          tone="red"
          onClick={() => setTab("awaiting")}
        />
        <StatTile
          label="Paid in full"
          value={counts.paid}
          active={tab === "paid"}
          tone="green"
          onClick={() => setTab("paid")}
        />
      </div>

      {notice && (
        <div className="p-3 bg-green-50 border border-green-300 text-green-800 text-sm rounded-lg">
          {notice}
        </div>
      )}
      {error && (
        <div className="p-3 bg-red-50 border border-red-300 text-red-800 text-sm rounded-lg">
          {error}
        </div>
      )}

      {/* Tabs + search + bulk */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="flex flex-wrap gap-1 bg-white border border-neutral-300 rounded-lg p-1">
          {TABS.map((t) => {
            const count =
              t.key === "all" ? undefined : (counts as Record<string, number>)[t.key];
            return (
              <button
                key={t.key}
                type="button"
                onClick={() => setTab(t.key)}
                className={`px-3 py-1.5 text-sm font-semibold rounded-md transition-colors ${
                  tab === t.key
                    ? "bg-primary text-white"
                    : "text-neutral-700 hover:bg-neutral-100"
                }`}
              >
                {t.label}
                {count !== undefined && count > 0 ? ` (${count})` : ""}
              </button>
            );
          })}
        </div>
        <input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search by name or email…"
          className="flex-1 min-w-[180px] px-3 py-2 text-sm bg-white border border-neutral-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary"
        />
        {tab === "to_invoice" && counts.to_invoice > 0 && (
          <button
            type="button"
            disabled={busy === "bulk"}
            onClick={sendAllAllocated}
            className="px-4 py-2 text-sm font-bold text-white bg-primary rounded-lg hover:bg-primary-dark disabled:opacity-50"
          >
            Send all {counts.to_invoice} invoices
          </button>
        )}
      </div>

      {/* Groups */}
      {visibleGroups.length === 0 ? (
        <div className="bg-white border border-dashed border-neutral-300 rounded-xl py-16 text-center text-neutral-500">
          {search
            ? "No one matches your search."
            : "Nothing here right now. 🎉"}
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          {visibleGroups.map((g) => (
            <GroupCard
              key={g.id}
              g={g}
              accommodations={accommodations}
              accName={accName}
              alloc={alloc[g.id] ?? {}}
              allAllocated={allFullWeekAllocated(g)}
              busy={busy}
              onSetAlloc={(camperId, code) => setCamperAlloc(g.id, camperId, code)}
              onSave={() => saveAllocation(g)}
              onInvoice={() => sendInvoice(g)}
              onResend={() => resendInvoice(g)}
              onExtend={() => extendDue(g)}
              onRelease={() => releaseGroup(g)}
            />
          ))}
        </div>
      )}

      {/* Advanced */}
      <details className="text-sm text-neutral-500 mt-2">
        <summary className="cursor-pointer select-none hover:text-neutral-700">
          Advanced
        </summary>
        <div className="mt-3 flex flex-col gap-2">
          <p>
            Overdue invoices are cancelled automatically every day. Use this
            only if you want to do it right now.
          </p>
          <button
            type="button"
            onClick={runSweep}
            disabled={busy === "sweep"}
            className="self-start px-3 py-2 text-sm font-semibold text-neutral-700 bg-white border border-neutral-300 rounded-lg hover:bg-neutral-100 disabled:opacity-50"
          >
            Release all overdue now
          </button>
        </div>
      </details>
    </div>
  );
}

function StatTile({
  label,
  value,
  tone,
  active,
  onClick,
}: {
  label: string;
  value: number;
  tone: "amber" | "blue" | "violet" | "red" | "green";
  active?: boolean;
  onClick: () => void;
}) {
  const tones: Record<string, string> = {
    amber: "text-amber-700",
    blue: "text-blue-700",
    violet: "text-violet-700",
    red: "text-red-700",
    green: "text-green-700",
  };
  return (
    <button
      type="button"
      onClick={onClick}
      className={`text-left bg-white border rounded-xl p-4 transition-all hover:shadow-sm ${
        active ? "border-primary ring-1 ring-primary" : "border-neutral-300"
      }`}
    >
      <p className={`text-3xl font-extrabold ${tones[tone]}`}>{value}</p>
      <p className="text-xs font-semibold text-neutral-600 mt-1 leading-tight">
        {label}
      </p>
    </button>
  );
}

function GroupCard({
  g,
  accommodations,
  accName,
  alloc,
  allAllocated,
  busy,
  onSetAlloc,
  onSave,
  onInvoice,
  onResend,
  onExtend,
  onRelease,
}: {
  g: AdminGroup;
  accommodations: AdminAccommodation[];
  accName: (code: string | null | undefined) => string;
  alloc: AllocState;
  allAllocated: boolean;
  busy: string | null;
  onSetAlloc: (camperId: string, code: string) => void;
  onSave: () => void;
  onInvoice: () => void;
  onResend: () => void;
  onExtend: () => void;
  onRelease: () => void;
}) {
  const fw = fullWeekCampers(g);
  const cat = categorize(g);
  const overdue = isOverdue(g);
  const badge = statusBadge(g);
  const canEditAlloc = cat === "to_allocate";

  return (
    <article
      className={`bg-white border rounded-xl overflow-hidden ${
        overdue ? "border-red-300" : "border-neutral-300"
      }`}
    >
      {/* Header */}
      <div className="flex flex-wrap items-start justify-between gap-3 p-4 sm:p-5 border-b border-neutral-200">
        <div className="min-w-0">
          <p className="text-lg font-bold text-neutral-800">
            {g.contact_first_name} {g.contact_last_name}
          </p>
          <p className="text-sm text-neutral-600 break-all">
            {g.contact_email}
            {g.contact_phone ? ` · ${g.contact_phone}` : ""}
          </p>
          <p className="text-xs text-neutral-400 mt-1">
            {g.campers.length} {g.campers.length === 1 ? "person" : "people"}
            {" · Deposit "}
            {g.payment_status === "paid" ? (
              <span className="text-green-700 font-semibold">
                paid {formatPence(g.total_amount_pence, g.currency)}
              </span>
            ) : (
              <span className="text-neutral-500">{g.payment_status}</span>
            )}
          </p>
        </div>
        <span
          className={`flex-none px-3 py-1 text-xs font-bold rounded-full border ${badge.className}`}
        >
          {badge.label}
        </span>
      </div>

      <div className="p-4 sm:p-5">
        {fw.length === 0 ? (
          <p className="text-sm text-neutral-500 italic">
            Day pass only — no balance invoice needed.
          </p>
        ) : canEditAlloc ? (
          <div className="flex flex-col gap-3">
            <p className="text-sm font-semibold text-neutral-700">
              Step 1 — choose accommodation for each person
            </p>
            <div className="flex flex-col divide-y divide-neutral-100 border border-neutral-200 rounded-lg">
              {fw.map((c) => {
                const selected = alloc[c.id] ?? "";
                const childAge = c.age <= MAX_CHILD_ACCOMMODATION_AGE;
                return (
                  <div
                    key={c.id}
                    className="flex flex-wrap items-center gap-3 p-3"
                  >
                    <div className="min-w-[140px] flex-1">
                      <p className="font-medium text-neutral-800">
                        {c.first_name} {c.last_name}
                      </p>
                      <p className="text-xs text-neutral-500">
                        Age {c.age}
                        {c.accommodation_first_choice
                          ? ` · prefers ${accName(c.accommodation_first_choice)}`
                          : ""}
                      </p>
                    </div>
                    <select
                      value={selected}
                      onChange={(e) => onSetAlloc(c.id, e.target.value)}
                      className="px-3 py-2 text-sm bg-white border border-neutral-300 rounded-lg min-w-[200px] focus:outline-none focus:ring-2 focus:ring-primary"
                    >
                      <option value="">— Choose —</option>
                      {accommodations.map((a) => {
                        const disabled =
                          a.code === ACCOMMODATION_CHILD_CODE && !childAge;
                        return (
                          <option
                            key={a.code}
                            value={a.code}
                            disabled={disabled}
                          >
                            {a.display_name}
                            {a.stripe_price_id ? "" : " (price not set)"}
                          </option>
                        );
                      })}
                    </select>
                    {c.age < MIN_DEPOSIT_AGE &&
                      selected === ACCOMMODATION_CHILD_CODE && (
                        <span className="text-xs text-neutral-500">
                          Under {MIN_DEPOSIT_AGE}: no balance to pay
                        </span>
                      )}
                  </div>
                );
              })}
            </div>
            <div>
              <button
                type="button"
                disabled={busy === `alloc-${g.id}` || !allAllocated}
                onClick={onSave}
                className="px-5 py-2.5 text-sm font-bold text-white bg-neutral-800 rounded-lg hover:bg-neutral-700 disabled:opacity-40"
              >
                {busy === `alloc-${g.id}` ? "Saving…" : "Save accommodation"}
              </button>
              {!allAllocated && (
                <span className="ml-3 text-xs text-neutral-500">
                  Choose accommodation for everyone to continue.
                </span>
              )}
            </div>
          </div>
        ) : (
          // Allocated / invoiced / paid → read-only summary of who's where
          <div className="flex flex-col gap-3">
            <div className="flex flex-col divide-y divide-neutral-100 border border-neutral-200 rounded-lg">
              {fw.map((c) => (
                <div
                  key={c.id}
                  className="flex items-center justify-between gap-3 p-3"
                >
                  <span className="font-medium text-neutral-800">
                    {c.first_name} {c.last_name}{" "}
                    <span className="text-xs text-neutral-400">
                      (age {c.age})
                    </span>
                  </span>
                  <span className="text-sm text-neutral-700 font-semibold">
                    {accName(c.allocated_accommodation_code)}
                  </span>
                </div>
              ))}
            </div>

            {cat === "to_invoice" && (
              <div className="flex flex-wrap items-center gap-3">
                <button
                  type="button"
                  disabled={busy === `inv-${g.id}`}
                  onClick={onInvoice}
                  className="px-5 py-2.5 text-sm font-bold text-white bg-primary rounded-lg hover:bg-primary-dark disabled:opacity-50"
                >
                  {busy === `inv-${g.id}`
                    ? "Sending…"
                    : "Send balance invoice"}
                </button>
                <span className="text-xs text-neutral-500">
                  Stripe will email a secure payment link.
                </span>
              </div>
            )}

            {cat === "awaiting" && (
              <div className="flex flex-col gap-2">
                <p
                  className={`text-sm ${
                    overdue ? "text-red-700 font-semibold" : "text-neutral-600"
                  }`}
                >
                  Invoice sent · due {formatDate(g.invoice_due_at)}
                  {overdue ? " · OVERDUE" : ""}
                </p>
                <div className="flex flex-wrap gap-2">
                  <button
                    type="button"
                    disabled={busy === `res-${g.id}`}
                    onClick={onResend}
                    className="px-4 py-2 text-sm font-semibold text-neutral-700 bg-white border border-neutral-300 rounded-lg hover:bg-neutral-100"
                  >
                    Re-send reminder
                  </button>
                  <button
                    type="button"
                    disabled={busy === `ext-${g.id}`}
                    onClick={onExtend}
                    className="px-4 py-2 text-sm font-semibold text-neutral-700 bg-white border border-neutral-300 rounded-lg hover:bg-neutral-100"
                  >
                    Give more time
                  </button>
                  <button
                    type="button"
                    disabled={busy === `rel-${g.id}`}
                    onClick={onRelease}
                    className="px-4 py-2 text-sm font-semibold text-red-700 bg-white border border-red-200 rounded-lg hover:bg-red-50"
                  >
                    Cancel & release
                  </button>
                </div>
              </div>
            )}

            {cat === "paid" && (
              <p className="text-sm text-green-700 font-semibold">
                ✓ Balance paid in full
                {g.balance_paid_at ? ` on ${formatDate(g.balance_paid_at)}` : ""}
                .
              </p>
            )}
          </div>
        )}
      </div>
    </article>
  );
}

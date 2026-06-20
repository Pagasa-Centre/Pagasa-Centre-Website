"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  adminApi,
  openAdminEventStream,
  type AdminAccommodation,
  type AdminAccommodationUnit,
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

type AllocState = Record<string, string>; // camperId -> accommodation tier code
type UnitAllocState = Record<string, string>; // camperId -> unit code

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

type SortKey = "date_desc" | "date_asc" | "name_asc" | "name_desc";

const SORT_OPTIONS: { key: SortKey; label: string }[] = [
  { key: "date_desc", label: "Newest first" },
  { key: "date_asc", label: "Oldest first" },
  { key: "name_asc", label: "Name A–Z" },
  { key: "name_desc", label: "Name Z–A" },
];

// Sort by surname, then first name, case-insensitively.
function compareByName(a: AdminGroup, b: AdminGroup): number {
  return (
    a.contact_last_name.localeCompare(b.contact_last_name, undefined, {
      sensitivity: "base",
    }) ||
    a.contact_first_name.localeCompare(b.contact_first_name, undefined, {
      sensitivity: "base",
    })
  );
}

function compareByDate(a: AdminGroup, b: AdminGroup): number {
  return new Date(a.created_at).getTime() - new Date(b.created_at).getTime();
}

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

function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return "—";
  try {
    return new Date(iso).toLocaleString("en-GB", {
      day: "numeric",
      month: "short",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return iso;
  }
}

const LAST_ACTION_LABELS: Record<string, string> = {
  allocate: "Allocated",
  unallocate: "Unallocated",
  invoice_sent: "Invoiced",
  invoice_resent: "Invoice re-sent",
  release: "Released",
  extend_due: "Due date extended",
  contact_updated: "Contact updated",
  balance_paid: "Balance paid",
};

function formatLastAction(g: AdminGroup): string | null {
  if (!g.last_action_by || !g.last_action_at) return null;
  const label =
    (g.last_action && LAST_ACTION_LABELS[g.last_action]) ||
    g.last_action ||
    "Updated";
  return `${label} by ${g.last_action_by} · ${formatDateTime(g.last_action_at)}`;
}

export default function AdminDashboard() {
  const router = useRouter();
  const [groups, setGroups] = useState<AdminGroup[]>([]);
  const [accommodations, setAccommodations] = useState<AdminAccommodation[]>([]);
  const [units, setUnits] = useState<AdminAccommodationUnit[]>([]);
  const [registrationsOpen, setRegistrationsOpen] = useState<boolean | null>(
    null,
  );
  const [alloc, setAlloc] = useState<Record<string, AllocState>>({});
  const [unitAlloc, setUnitAlloc] = useState<Record<string, UnitAllocState>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [tab, setTab] = useState<TabKey>("to_allocate");
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState<SortKey>("date_desc");
  const [actorName, setActorName] = useState<string | null>(null);
  const [streamReconnecting, setStreamReconnecting] = useState(false);
  // Group IDs whose (already-saved) allocation is being re-edited.
  const [editing, setEditing] = useState<Record<string, boolean>>({});

  const accName = useCallback(
    (code: string | null | undefined): string => {
      if (!code) return "—";
      return accommodations.find((a) => a.code === code)?.display_name ?? code;
    },
    [accommodations],
  );

  const unitName = useCallback(
    (code: string | null | undefined): string => {
      if (!code) return "";
      return units.find((u) => u.code === code)?.display_name ?? code;
    },
    [units],
  );

  const load = useCallback(async () => {
    setError(null);
    try {
      const [reg, acc, unitRes, cfg] = await Promise.all([
        adminApi.listRegistrations(),
        adminApi.accommodations(),
        adminApi.accommodationUnits(),
        adminApi.campConfig(),
      ]);
      setGroups(reg.groups);
      setAccommodations(acc.accommodations);
      setUnits(unitRes.units);
      setRegistrationsOpen(cfg.registrations_open);
      const next: Record<string, AllocState> = {};
      const nextUnits: Record<string, UnitAllocState> = {};
      for (const g of reg.groups) {
        const m: AllocState = {};
        const um: UnitAllocState = {};
        for (const c of g.campers) {
          if (c.allocated_accommodation_code) {
            m[c.id] = c.allocated_accommodation_code;
          } else if (
            c.attendance_type === "full_week" &&
            c.accommodation_first_choice
          ) {
            m[c.id] = c.accommodation_first_choice;
          }
          if (c.allocated_unit_code) {
            um[c.id] = c.allocated_unit_code;
          }
        }
        next[g.id] = m;
        nextUnits[g.id] = um;
      }
      setAlloc(next);
      setUnitAlloc(nextUnits);
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
      .then((session) => {
        setActorName(session.name);
        return load();
      })
      .catch(() => router.replace("/admin/login"));
  }, [load, router]);

  useEffect(() => {
    let debounce: ReturnType<typeof setTimeout> | null = null;
    const es = openAdminEventStream({
      onOpen: () => {
        setStreamReconnecting(false);
        void load();
      },
      onError: () => setStreamReconnecting(true),
      onEvent: () => {
        if (debounce) clearTimeout(debounce);
        debounce = setTimeout(() => void load(), 400);
      },
    });
    return () => {
      if (debounce) clearTimeout(debounce);
      es?.close();
    };
  }, [load]);

  async function handleAdminError(err: unknown, fallback: string) {
    setError(err instanceof AdminApiError ? err.detail.message : fallback);
    if (err instanceof AdminApiError && err.status === 409) {
      await load();
    }
  }

  function setCamperAlloc(groupId: string, camperId: string, code: string) {
    setAlloc((prev) => ({
      ...prev,
      [groupId]: { ...prev[groupId], [camperId]: code },
    }));
    // Changing tier invalidates the previously chosen unit.
    setUnitAlloc((prev) => {
      const groupUnits = { ...(prev[groupId] ?? {}) };
      delete groupUnits[camperId];
      return { ...prev, [groupId]: groupUnits };
    });
  }

  function setCamperUnit(groupId: string, camperId: string, unitCode: string) {
    setUnitAlloc((prev) => ({
      ...prev,
      [groupId]: { ...prev[groupId], [camperId]: unitCode },
    }));
  }

  // Pre-fill the dropdowns with whatever is currently saved for the group.
  function allocFromSaved(g: AdminGroup): AllocState {
    const m: AllocState = {};
    for (const c of g.campers) {
      if (c.allocated_accommodation_code) {
        m[c.id] = c.allocated_accommodation_code;
      }
    }
    return m;
  }

  function unitAllocFromSaved(g: AdminGroup): UnitAllocState {
    const m: UnitAllocState = {};
    for (const c of g.campers) {
      if (c.allocated_unit_code) {
        m[c.id] = c.allocated_unit_code;
      }
    }
    return m;
  }

  function startEdit(g: AdminGroup) {
    setAlloc((prev) => ({ ...prev, [g.id]: allocFromSaved(g) }));
    setUnitAlloc((prev) => ({ ...prev, [g.id]: unitAllocFromSaved(g) }));
    setEditing((e) => ({ ...e, [g.id]: true }));
  }

  function cancelEdit(g: AdminGroup) {
    setAlloc((prev) => ({ ...prev, [g.id]: allocFromSaved(g) }));
    setUnitAlloc((prev) => ({ ...prev, [g.id]: unitAllocFromSaved(g) }));
    setEditing((e) => {
      const next = { ...e };
      delete next[g.id];
      return next;
    });
  }

  function allFullWeekAllocated(g: AdminGroup): boolean {
    const fw = fullWeekCampers(g);
    if (fw.length === 0) return false;
    const state = alloc[g.id] ?? {};
    return fw.every((c) => !!state[c.id]);
  }

  // Projected per-unit occupancy if group g's current draft were saved.
  // Baseline = every OTHER group's saved units (g's own saved units are
  // replaced by its draft), plus g's draft selections.
  function overCapacityUnitsAfterSave(
    g: AdminGroup,
  ): { name: string; used: number; cap: number }[] {
    const counts: Record<string, number> = {};
    for (const other of groups) {
      if (other.id === g.id) continue;
      for (const c of other.campers) {
        const code = c.allocated_unit_code;
        if (code) counts[code] = (counts[code] ?? 0) + 1;
      }
    }
    const draft = unitAlloc[g.id] ?? {};
    for (const c of fullWeekCampers(g)) {
      const code = draft[c.id];
      if (code) counts[code] = (counts[code] ?? 0) + 1;
    }
    const over: { name: string; used: number; cap: number }[] = [];
    for (const [code, used] of Object.entries(counts)) {
      const u = units.find((x) => x.code === code);
      if (u && used > u.capacity) {
        over.push({ name: u.display_name, used, cap: u.capacity });
      }
    }
    return over;
  }

  async function saveAllocation(g: AdminGroup) {
    const over = overCapacityUnitsAfterSave(g);
    if (over.length > 0) {
      const lines = over
        .map((o) => `• ${o.name}: ${o.used} people, but it only sleeps ${o.cap}`)
        .join("\n");
      if (
        !confirm(
          `⚠️ Some accommodation will be over its limit:\n\n${lines}\n\n` +
            `This usually means too many people are sharing one unit. ` +
            `Do you want to save anyway?`,
        )
      ) {
        return;
      }
    }
    setBusy(`alloc-${g.id}`);
    setError(null);
    setNotice(null);
    try {
      const campers: AllocateCamper[] = fullWeekCampers(g).map((c) => {
        const payload: AllocateCamper = {
          camper_id: c.id,
          allocated_accommodation_code: alloc[g.id]?.[c.id] ?? "",
        };
        const unit = unitAlloc[g.id]?.[c.id];
        if (unit) payload.allocated_unit_code = unit;
        return payload;
      });
      await adminApi.saveAllocation(g.id, campers, g.version);
      setEditing((e) => {
        const next = { ...e };
        delete next[g.id];
        return next;
      });
      setNotice(`Accommodation saved for ${g.contact_first_name}'s group.`);
      await load();
    } catch (err) {
      await handleAdminError(err, "Save failed.");
    } finally {
      setBusy(null);
    }
  }

  async function resetAllocation(g: AdminGroup) {
    if (
      !confirm(
        `Reset ${g.contact_first_name} ${g.contact_last_name}'s group back to "Needs accommodation"?\n\nThis clears their saved allocation. No invoice has been sent, so nothing is charged.`,
      )
    ) {
      return;
    }
    setBusy(`reset-${g.id}`);
    setError(null);
    setNotice(null);
    try {
      await adminApi.unallocate(g.id, g.version);
      setEditing((e) => {
        const next = { ...e };
        delete next[g.id];
        return next;
      });
      setNotice(`${g.contact_first_name}'s group moved back to Needs accommodation.`);
      await load();
    } catch (err) {
      await handleAdminError(err, "Reset failed.");
    } finally {
      setBusy(null);
    }
  }

  async function sendInvoice(g: AdminGroup) {
    setBusy(`inv-${g.id}`);
    setError(null);
    setNotice(null);
    try {
      await adminApi.sendInvoice(g.id, g.version);
      setNotice(`Invoice emailed to ${g.contact_email}.`);
      await load();
    } catch (err) {
      await handleAdminError(err, "Invoice failed.");
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
      await handleAdminError(err, "Bulk send failed.");
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
      await adminApi.release(g.id, g.version);
      setNotice(`Released ${g.contact_first_name}'s accommodation.`);
      await load();
    } catch (err) {
      await handleAdminError(err, "Release failed.");
    } finally {
      setBusy(null);
    }
  }

  async function resendInvoice(g: AdminGroup) {
    setBusy(`res-${g.id}`);
    setError(null);
    setNotice(null);
    try {
      await adminApi.resendInvoice(g.id, g.version);
      setNotice(`Reminder re-sent to ${g.contact_email}.`);
    } catch (err) {
      await handleAdminError(err, "Resend failed.");
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
      await adminApi.extendDue(g.id, due.toISOString(), g.version);
      setNotice(`New due date set to ${formatDate(due.toISOString())}.`);
      await load();
    } catch (err) {
      await handleAdminError(err, "Extend failed.");
    } finally {
      setBusy(null);
    }
  }

  async function saveContact(
    g: AdminGroup,
    data: {
      first_name: string;
      last_name: string;
      email: string;
      phone: string;
      resend_confirmation: boolean;
    },
  ): Promise<boolean> {
    setBusy(`contact-${g.id}`);
    setError(null);
    setNotice(null);
    try {
      await adminApi.updateContact(g.id, data);
      setNotice(
        data.resend_confirmation
          ? `Contact details updated. Confirmation email re-sent to ${data.email}.`
          : `Contact details updated for ${data.first_name} ${data.last_name}.`,
      );
      await load();
      return true;
    } catch (err) {
      setError(
        err instanceof AdminApiError
          ? err.detail.message
          : "Could not update contact details.",
      );
      return false;
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

  async function toggleRegistrations() {
    if (registrationsOpen === null) return;
    const next = !registrationsOpen;
    const verb = next ? "OPEN" : "CLOSE";
    if (
      !window.confirm(
        next
          ? "Open camp registration to the public? People will be able to sign up immediately."
          : "Close camp registration? The website will show a 'Registrations Closed' notice and no new sign-ups will be accepted.",
      )
    ) {
      return;
    }
    setBusy("reg-toggle");
    setError(null);
    setNotice(null);
    try {
      const res = await adminApi.setRegistrationsOpen(next);
      setRegistrationsOpen(res.registrations_open);
      setNotice(
        res.registrations_open
          ? "Registration is now OPEN — the public can sign up."
          : "Registration is now CLOSED — the public sees the closed notice.",
      );
    } catch (err) {
      setError(
        err instanceof AdminApiError
          ? err.detail.message
          : `Could not ${verb.toLowerCase()} registration.`,
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

  // How many beds are currently held per accommodation code. A camper holds a
  // spot once their allocation is saved; releasing a group clears the code, so
  // this naturally only counts live allocations.
  const usage = useMemo(() => {
    const used: Record<string, number> = {};
    for (const g of groups) {
      for (const c of g.campers) {
        const code = c.allocated_accommodation_code;
        if (code) used[code] = (used[code] ?? 0) + 1;
      }
    }
    return used;
  }, [groups]);

  const unitUsage = useMemo(() => {
    const byUnit: Record<string, { used: number; groupIds: Set<string> }> = {};
    for (const g of groups) {
      for (const c of g.campers) {
        const code = c.allocated_unit_code;
        if (!code) continue;
        if (!byUnit[code]) byUnit[code] = { used: 0, groupIds: new Set() };
        byUnit[code].used++;
        byUnit[code].groupIds.add(g.id);
      }
    }
    return byUnit;
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
        switch (sort) {
          case "date_asc":
            return compareByDate(a, b);
          case "date_desc":
            return compareByDate(b, a);
          case "name_asc":
            return compareByName(a, b);
          case "name_desc":
            return compareByName(b, a);
          default:
            return 0;
        }
      });
  }, [groups, tab, search, sort]);

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
          {actorName && (
            <p className="text-xs text-neutral-500 mt-1">
              Signed in as{" "}
              <span className="font-semibold text-neutral-700">{actorName}</span>
              {streamReconnecting && (
                <span className="ml-2 text-amber-600">· Reconnecting…</span>
              )}
            </p>
          )}
        </div>
        <div className="flex items-center gap-2">
          <Link
            href="/admin/activity"
            className="px-3 py-2 text-sm font-semibold text-neutral-700 bg-white border border-neutral-300 rounded-lg hover:bg-neutral-100"
          >
            Activity log
          </Link>
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

      {/* Registration open/closed control */}
      {registrationsOpen !== null && (
        <div
          className={`flex flex-wrap items-center justify-between gap-3 rounded-xl border p-4 sm:p-5 ${
            registrationsOpen
              ? "bg-green-50 border-green-200"
              : "bg-amber-50 border-amber-200"
          }`}
        >
          <div className="flex items-center gap-3">
            <span
              className={`flex-none w-2.5 h-2.5 rounded-full ${
                registrationsOpen ? "bg-green-500" : "bg-amber-500"
              }`}
              aria-hidden
            />
            <div>
              <p className="text-sm font-bold text-neutral-800">
                Public registration is{" "}
                {registrationsOpen ? "OPEN" : "CLOSED"}
              </p>
              <p className="text-xs text-neutral-600 mt-0.5">
                {registrationsOpen
                  ? "Anyone can sign up on the website right now."
                  : "The website shows a “Registrations Closed” notice — no new sign-ups are accepted."}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={toggleRegistrations}
            disabled={busy === "reg-toggle"}
            className={`px-4 py-2 text-sm font-semibold rounded-lg text-white disabled:opacity-60 ${
              registrationsOpen
                ? "bg-amber-600 hover:bg-amber-700"
                : "bg-green-600 hover:bg-green-700"
            }`}
          >
            {busy === "reg-toggle"
              ? "Saving…"
              : registrationsOpen
                ? "Close registration"
                : "Open registration"}
          </button>
        </div>
      )}

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

      {/* Accommodation availability */}
      <CapacityPanel
        accommodations={accommodations}
        units={units}
        usage={usage}
        unitUsage={unitUsage}
      />

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
        <label className="flex items-center gap-2 text-sm text-neutral-600">
          <span className="font-semibold text-neutral-700">Sort</span>
          <select
            value={sort}
            onChange={(e) => setSort(e.target.value as SortKey)}
            className="px-3 py-2 text-sm bg-white border border-neutral-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary"
          >
            {SORT_OPTIONS.map((o) => (
              <option key={o.key} value={o.key}>
                {o.label}
              </option>
            ))}
          </select>
        </label>
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
              units={units}
              accName={accName}
              unitName={unitName}
              alloc={alloc[g.id] ?? {}}
              unitAlloc={unitAlloc[g.id] ?? {}}
              allAllocated={allFullWeekAllocated(g)}
              busy={busy}
              isEditing={!!editing[g.id]}
              onUpdateContact={(data) => saveContact(g, data)}
              onSetAlloc={(camperId, code) => setCamperAlloc(g.id, camperId, code)}
              onSetUnit={(camperId, code) => setCamperUnit(g.id, camperId, code)}
              onSave={() => saveAllocation(g)}
              onEdit={() => startEdit(g)}
              onCancelEdit={() => cancelEdit(g)}
              onReset={() => resetAllocation(g)}
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

function CapacityPanel({
  accommodations,
  units,
  usage,
  unitUsage,
}: {
  accommodations: AdminAccommodation[];
  units: AdminAccommodationUnit[];
  usage: Record<string, number>;
  unitUsage: Record<string, { used: number; groupIds: Set<string> }>;
}) {
  if (accommodations.length === 0) return null;

  const unitsByTier = useMemo(() => {
    const m: Record<string, AdminAccommodationUnit[]> = {};
    for (const u of units) {
      if (!m[u.accommodation_code]) m[u.accommodation_code] = [];
      m[u.accommodation_code].push(u);
    }
    for (const list of Object.values(m)) {
      list.sort((a, b) => a.sort_order - b.sort_order || a.code.localeCompare(b.code));
    }
    return m;
  }, [units]);

  return (
    <div className="bg-white border border-neutral-300 rounded-xl p-4 sm:p-5">
      <p className="text-sm font-bold text-neutral-800 mb-3">
        Accommodation availability
      </p>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
        {accommodations.map((a) => {
          const used = usage[a.code] ?? 0;
          const cap = a.capacity ?? null;
          const hasLimit = cap !== null && cap !== undefined;
          const left = hasLimit ? Math.max(0, cap - used) : null;
          const full = hasLimit && used >= cap;
          const pct = hasLimit && cap > 0 ? Math.min(100, (used / cap) * 100) : 0;
          const tierUnits = unitsByTier[a.code] ?? [];

          let barColor = "bg-green-500";
          let tag = `${left} space${left === 1 ? "" : "s"} left`;
          let tagColor = "text-green-700 bg-green-100";
          if (!hasLimit) {
            tag = "No limit";
            tagColor = "text-neutral-600 bg-neutral-100";
          } else if (full) {
            barColor = "bg-red-500";
            tag = "Full";
            tagColor = "text-red-700 bg-red-100";
          } else if (left !== null && left <= Math.max(2, cap * 0.15)) {
            barColor = "bg-amber-500";
            tagColor = "text-amber-700 bg-amber-100";
          }

          return (
            <div
              key={a.code}
              className="border border-neutral-200 rounded-lg p-3"
            >
              <div className="flex items-center justify-between gap-2">
                <span className="text-sm font-semibold text-neutral-800 truncate">
                  {a.display_name}
                </span>
                <span
                  className={`flex-none text-[11px] font-bold px-2 py-0.5 rounded-full ${tagColor}`}
                >
                  {tag}
                </span>
              </div>
              <p className="text-xs text-neutral-500 mt-1">
                {hasLimit ? (
                  <>
                    <span className="font-bold text-neutral-800">{used}</span> of{" "}
                    {cap} allocated
                  </>
                ) : (
                  <>
                    <span className="font-bold text-neutral-800">{used}</span>{" "}
                    allocated
                  </>
                )}
              </p>
              {hasLimit && (
                <div className="mt-2 h-2 w-full rounded-full bg-neutral-100 overflow-hidden">
                  <div
                    className={`h-full rounded-full ${barColor}`}
                    style={{ width: `${pct}%` }}
                  />
                </div>
              )}
              {tierUnits.length > 0 && (
                <ul className="mt-3 flex flex-col gap-1.5 border-t border-neutral-100 pt-2">
                  {tierUnits.map((u) => {
                    const stats = unitUsage[u.code];
                    const uUsed = stats?.used ?? 0;
                    const over = uUsed > u.capacity;
                    const full = !over && uUsed >= u.capacity;
                    const mixed = (stats?.groupIds.size ?? 0) > 1;
                    let rowClass = "";
                    if (over) rowClass = "bg-red-50 border border-red-200";
                    else if (full) rowClass = "bg-green-50 border border-green-200";
                    return (
                      <li
                        key={u.code}
                        className={`flex flex-wrap items-center justify-between gap-1 text-[11px] rounded px-1.5 py-1 ${rowClass}`}
                      >
                        <span
                          className={
                            over
                              ? "text-red-800 font-semibold"
                              : full
                                ? "text-green-800 font-semibold"
                                : "text-neutral-600"
                          }
                        >
                          {u.display_name}{" "}
                          <span
                            className={over || full ? "opacity-70" : "text-neutral-400"}
                          >
                            (sleeps {u.capacity})
                          </span>
                        </span>
                        <span className="flex items-center gap-1">
                          <span
                            className={`font-semibold ${
                              over
                                ? "text-red-700"
                                : full
                                  ? "text-green-700"
                                  : "text-neutral-700"
                            }`}
                          >
                            {uUsed}/{u.capacity}
                          </span>
                          {over && (
                            <span className="font-bold text-red-700 bg-red-100 px-1.5 py-0.5 rounded">
                              Over capacity
                            </span>
                          )}
                          {full && (
                            <span className="font-bold text-green-700 bg-green-100 px-1.5 py-0.5 rounded">
                              Full
                            </span>
                          )}
                          {mixed && (
                            <span className="font-bold text-amber-800 bg-amber-100 px-1.5 py-0.5 rounded">
                              Mixed groups
                            </span>
                          )}
                        </span>
                      </li>
                    );
                  })}
                </ul>
              )}
            </div>
          );
        })}
      </div>
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

function tierHasUnits(
  tierCode: string,
  units: AdminAccommodationUnit[],
): boolean {
  return units.some((u) => u.accommodation_code === tierCode);
}

function unitsForTier(
  tierCode: string,
  units: AdminAccommodationUnit[],
): AdminAccommodationUnit[] {
  return units
    .filter((u) => u.accommodation_code === tierCode)
    .sort((a, b) => a.sort_order - b.sort_order || a.code.localeCompare(b.code));
}

function GroupCard({
  g,
  accommodations,
  units,
  accName,
  unitName,
  alloc,
  unitAlloc,
  allAllocated,
  busy,
  isEditing,
  onUpdateContact,
  onSetAlloc,
  onSetUnit,
  onSave,
  onEdit,
  onCancelEdit,
  onReset,
  onInvoice,
  onResend,
  onExtend,
  onRelease,
}: {
  g: AdminGroup;
  accommodations: AdminAccommodation[];
  units: AdminAccommodationUnit[];
  accName: (code: string | null | undefined) => string;
  unitName: (code: string | null | undefined) => string;
  alloc: AllocState;
  unitAlloc: UnitAllocState;
  allAllocated: boolean;
  busy: string | null;
  isEditing: boolean;
  onUpdateContact: (data: {
    first_name: string;
    last_name: string;
    email: string;
    phone: string;
    resend_confirmation: boolean;
  }) => Promise<boolean>;
  onSetAlloc: (camperId: string, code: string) => void;
  onSetUnit: (camperId: string, code: string) => void;
  onSave: () => void;
  onEdit: () => void;
  onCancelEdit: () => void;
  onReset: () => void;
  onInvoice: () => void;
  onResend: () => void;
  onExtend: () => void;
  onRelease: () => void;
}) {
  const fw = fullWeekCampers(g);
  const cat = categorize(g);
  const overdue = isOverdue(g);
  const badge = statusBadge(g);
  // Editable when the group still needs allocating, OR when the team has
  // explicitly chosen to edit an already-saved (but not yet invoiced) group.
  const canEditAlloc = cat === "to_allocate" || (cat === "to_invoice" && isEditing);

  const [contactOpen, setContactOpen] = useState(false);
  const [cFirst, setCFirst] = useState(g.contact_first_name);
  const [cLast, setCLast] = useState(g.contact_last_name);
  const [cEmail, setCEmail] = useState(g.contact_email);
  const [cPhone, setCPhone] = useState(g.contact_phone);
  const [cResend, setCResend] = useState(g.payment_status === "paid");
  const contactBusy = busy === `contact-${g.id}`;

  function openContact() {
    setCFirst(g.contact_first_name);
    setCLast(g.contact_last_name);
    setCEmail(g.contact_email);
    setCPhone(g.contact_phone);
    setCResend(g.payment_status === "paid");
    setContactOpen(true);
  }

  async function submitContact() {
    const ok = await onUpdateContact({
      first_name: cFirst,
      last_name: cLast,
      email: cEmail,
      phone: cPhone,
      resend_confirmation: cResend,
    });
    if (ok) setContactOpen(false);
  }

  return (
    <article
      className={`bg-white border rounded-xl overflow-hidden ${
        overdue ? "border-red-300" : "border-neutral-300"
      }`}
    >
      {/* Header */}
      <div className="p-4 sm:p-5 border-b border-neutral-200">
        <div className="flex flex-wrap items-start justify-between gap-3">
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
            <p className="text-xs text-neutral-400 mt-0.5">
              Registered {formatDateTime(g.created_at)}
            </p>
            {formatLastAction(g) && (
              <p className="text-xs text-neutral-500 mt-0.5 italic">
                {formatLastAction(g)}
              </p>
            )}
            {!contactOpen && (
              <button
                type="button"
                onClick={openContact}
                className="mt-2 text-xs font-semibold text-primary hover:underline"
              >
                Edit contact / fix email
              </button>
            )}
          </div>
          <span
            className={`flex-none px-3 py-1 text-xs font-bold rounded-full border ${badge.className}`}
          >
            {badge.label}
          </span>
        </div>

        {contactOpen && (
          <div className="mt-4 border-t border-neutral-200 pt-4 flex flex-col gap-3">
            <p className="text-sm font-semibold text-neutral-700">
              Edit contact details
            </p>
            <div className="grid sm:grid-cols-2 gap-3">
              <label className="flex flex-col gap-1 text-xs font-semibold text-neutral-600">
                First name
                <input
                  value={cFirst}
                  onChange={(e) => setCFirst(e.target.value)}
                  className="px-3 py-2 text-sm font-normal text-neutral-800 bg-white border border-neutral-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary"
                />
              </label>
              <label className="flex flex-col gap-1 text-xs font-semibold text-neutral-600">
                Last name
                <input
                  value={cLast}
                  onChange={(e) => setCLast(e.target.value)}
                  className="px-3 py-2 text-sm font-normal text-neutral-800 bg-white border border-neutral-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary"
                />
              </label>
              <label className="flex flex-col gap-1 text-xs font-semibold text-neutral-600">
                Email
                <input
                  type="email"
                  value={cEmail}
                  onChange={(e) => setCEmail(e.target.value)}
                  className="px-3 py-2 text-sm font-normal text-neutral-800 bg-white border border-neutral-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary"
                />
              </label>
              <label className="flex flex-col gap-1 text-xs font-semibold text-neutral-600">
                Phone
                <input
                  value={cPhone}
                  onChange={(e) => setCPhone(e.target.value)}
                  className="px-3 py-2 text-sm font-normal text-neutral-800 bg-white border border-neutral-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary"
                />
              </label>
            </div>
            {g.payment_status === "paid" ? (
              <label className="flex items-start gap-2 text-sm text-neutral-700">
                <input
                  type="checkbox"
                  checked={cResend}
                  onChange={(e) => setCResend(e.target.checked)}
                  className="mt-0.5"
                />
                <span>
                  Re-send the deposit confirmation email to this address
                </span>
              </label>
            ) : (
              <p className="text-xs text-neutral-500">
                The confirmation email can only be re-sent once the deposit is
                paid.
              </p>
            )}
            <div className="flex flex-wrap items-center gap-2">
              <button
                type="button"
                disabled={contactBusy}
                onClick={submitContact}
                className="px-5 py-2.5 text-sm font-bold text-white bg-neutral-800 rounded-lg hover:bg-neutral-700 disabled:opacity-40"
              >
                {contactBusy ? "Saving…" : "Save changes"}
              </button>
              <button
                type="button"
                disabled={contactBusy}
                onClick={() => setContactOpen(false)}
                className="px-4 py-2.5 text-sm font-semibold text-neutral-700 bg-white border border-neutral-300 rounded-lg hover:bg-neutral-100"
              >
                Cancel
              </button>
            </div>
          </div>
        )}
      </div>

      <div className="p-4 sm:p-5">
        {fw.length === 0 ? (
          <p className="text-sm text-neutral-500 italic">
            Day pass only — no balance invoice needed.
          </p>
        ) : canEditAlloc ? (
          <div className="flex flex-col gap-3">
            <p className="text-sm font-semibold text-neutral-700">
              {isEditing
                ? "Edit accommodation — change any choice, then save"
                : "Step 1 — choose accommodation for each person"}
            </p>
            <div className="flex flex-col divide-y divide-neutral-100 border border-neutral-200 rounded-lg">
              {fw.map((c) => {
                const selected = alloc[c.id] ?? "";
                const selectedUnit = unitAlloc[c.id] ?? "";
                const childAge = c.age <= MAX_CHILD_ACCOMMODATION_AGE;
                const showUnits = selected && tierHasUnits(selected, units);
                const tierUnits = showUnits ? unitsForTier(selected, units) : [];
                return (
                  <div
                    key={c.id}
                    className="flex flex-wrap items-center gap-3 p-3"
                  >
                    <div className="min-w-[160px] flex-1">
                      <p className="font-medium text-neutral-800">
                        {c.first_name} {c.last_name}
                        <span className="ml-2 text-xs font-semibold text-neutral-600 bg-neutral-100 px-2 py-0.5 rounded-full">
                          Age {c.age}
                        </span>
                      </p>
                      <p className="text-xs text-neutral-500 mt-1">
                        <span className="font-semibold">1st:</span>{" "}
                        {c.accommodation_first_choice
                          ? accName(c.accommodation_first_choice)
                          : "—"}
                        <span className="mx-1.5 text-neutral-300">|</span>
                        <span className="font-semibold">2nd:</span>{" "}
                        {c.accommodation_second_choice
                          ? accName(c.accommodation_second_choice)
                          : "—"}
                      </p>
                    </div>
                    <select
                      value={selected}
                      onChange={(e) => onSetAlloc(c.id, e.target.value)}
                      className="px-3 py-2 text-sm bg-white border border-neutral-300 rounded-lg min-w-[180px] focus:outline-none focus:ring-2 focus:ring-primary"
                    >
                      <option value="">— Choose tier —</option>
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
                    {showUnits && (
                      <select
                        value={selectedUnit}
                        onChange={(e) => onSetUnit(c.id, e.target.value)}
                        className="px-3 py-2 text-sm bg-white border border-neutral-300 rounded-lg min-w-[180px] focus:outline-none focus:ring-2 focus:ring-primary"
                      >
                        <option value="">— Unit (optional) —</option>
                        {tierUnits.map((u) => (
                          <option key={u.code} value={u.code}>
                            {u.display_name} (sleeps {u.capacity})
                          </option>
                        ))}
                      </select>
                    )}
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
            <div className="flex flex-wrap items-center gap-2">
              <button
                type="button"
                disabled={busy === `alloc-${g.id}` || !allAllocated}
                onClick={onSave}
                className="px-5 py-2.5 text-sm font-bold text-white bg-neutral-800 rounded-lg hover:bg-neutral-700 disabled:opacity-40"
              >
                {busy === `alloc-${g.id}` ? "Saving…" : "Save accommodation"}
              </button>
              {isEditing && (
                <button
                  type="button"
                  disabled={busy === `alloc-${g.id}`}
                  onClick={onCancelEdit}
                  className="px-4 py-2.5 text-sm font-semibold text-neutral-700 bg-white border border-neutral-300 rounded-lg hover:bg-neutral-100"
                >
                  Cancel
                </button>
              )}
              {!allAllocated && (
                <span className="text-xs text-neutral-500">
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
                    <span className="ml-1 text-xs font-semibold text-neutral-600 bg-neutral-100 px-2 py-0.5 rounded-full">
                      Age {c.age}
                    </span>
                  </span>
                  <span className="text-sm text-neutral-700 font-semibold text-right">
                    {accName(c.allocated_accommodation_code)}
                    {c.allocated_unit_code && (
                      <span className="block text-xs font-medium text-neutral-500">
                        {unitName(c.allocated_unit_code)}
                      </span>
                    )}
                  </span>
                </div>
              ))}
            </div>

            {cat === "to_invoice" && (
              <div className="flex flex-wrap items-center gap-2">
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
                <button
                  type="button"
                  onClick={onEdit}
                  className="px-4 py-2.5 text-sm font-semibold text-neutral-700 bg-white border border-neutral-300 rounded-lg hover:bg-neutral-100"
                >
                  Edit allocation
                </button>
                <button
                  type="button"
                  disabled={busy === `reset-${g.id}`}
                  onClick={onReset}
                  className="px-4 py-2.5 text-sm font-semibold text-red-700 bg-white border border-red-200 rounded-lg hover:bg-red-50"
                >
                  Reset allocation
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

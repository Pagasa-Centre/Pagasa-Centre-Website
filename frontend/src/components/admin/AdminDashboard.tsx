"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import {
  adminApi,
  type AdminAccommodation,
  type AdminGroup,
  type AllocateCamper,
  AdminApiError,
} from "@/lib/admin-api";
import { ACCOMMODATION_CHILD_CODE, MIN_DEPOSIT_AGE } from "@/lib/camp";

type AllocState = Record<string, string>; // camperId -> accommodation code

function billingLabel(status: string): string {
  switch (status) {
    case "none":
      return "Not allocated";
    case "allocated":
      return "Allocated (not invoiced)";
    case "invoiced":
      return "Invoice sent";
    case "balance_paid":
      return "Balance paid";
    case "released":
      return "Released";
    default:
      return status;
  }
}

function formatDue(iso: string | null | undefined): string {
  if (!iso) return "—";
  try {
    return new Date(iso).toLocaleString("en-GB", {
      dateStyle: "medium",
      timeStyle: "short",
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
  const [busy, setBusy] = useState<string | null>(null);
  const [filter, setFilter] = useState<"paid" | "all">("paid");

  const load = useCallback(async () => {
    setError(null);
    try {
      const [reg, acc] = await Promise.all([
        adminApi.listRegistrations(
          filter === "paid" ? { status: "paid" } : undefined,
        ),
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
      setError("Failed to load registrations.");
    } finally {
      setLoading(false);
    }
  }, [filter, router]);

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

  function fullWeekCampers(g: AdminGroup) {
    return g.campers.filter((c) => c.attendance_type === "full_week");
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
    try {
      const campers: AllocateCamper[] = fullWeekCampers(g).map((c) => ({
        camper_id: c.id,
        allocated_accommodation_code: alloc[g.id]?.[c.id] ?? "",
      }));
      await adminApi.saveAllocation(g.id, campers);
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
    try {
      await adminApi.sendInvoice(g.id);
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
      .filter((g) => g.billing_status === "allocated")
      .map((g) => g.id);
    if (ids.length === 0) return;
    setBusy("bulk");
    setError(null);
    try {
      const res = await adminApi.sendInvoiceBulk(ids);
      if (res && typeof res === "object" && "errors" in res) {
        const errs = (res as { errors: Record<string, string> }).errors;
        setError(
          `Some invoices failed: ${Object.keys(errs).length} group(s). Check logs.`,
        );
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
        `Release allocation for ${g.contact_email}? This voids the open invoice.`,
      )
    ) {
      return;
    }
    setBusy(`rel-${g.id}`);
    try {
      await adminApi.release(g.id);
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
    try {
      await adminApi.resendInvoice(g.id);
    } catch (err) {
      setError(
        err instanceof AdminApiError ? err.detail.message : "Resend failed.",
      );
    } finally {
      setBusy(null);
    }
  }

  async function extendDue(g: AdminGroup) {
    const days = prompt("Extend due date by how many days from today?", "7");
    if (!days) return;
    const n = parseInt(days, 10);
    if (!Number.isFinite(n) || n <= 0) return;
    const due = new Date();
    due.setDate(due.getDate() + n);
    setBusy(`ext-${g.id}`);
    try {
      await adminApi.extendDue(g.id, due.toISOString());
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
    setBusy("sweep");
    try {
      const res = await adminApi.sweep();
      alert(`Released ${res.released} overdue group(s).`);
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

  const allocatedReady = groups.filter((g) => g.billing_status === "allocated");

  if (loading) {
    return <p className="text-center text-neutral-600 py-20">Loading…</p>;
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-extrabold text-neutral-900">
            Camp registrations
          </h1>
          <p className="text-sm text-neutral-600 mt-1">
            Allocate accommodation, then send Stripe balance invoices (15-day
            due). Unpaid allocations are released automatically after the due
            date.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={() => setFilter((f) => (f === "paid" ? "all" : "paid"))}
            className="px-4 py-2 border border-neutral-300 text-xs font-bold uppercase tracking-widest hover:border-primary"
          >
            {filter === "paid" ? "Show all" : "Deposit paid only"}
          </button>
          <button
            type="button"
            disabled={allocatedReady.length === 0 || busy === "bulk"}
            onClick={sendAllAllocated}
            className="px-4 py-2 bg-primary text-white text-xs font-bold uppercase tracking-widest hover:bg-primary-dark disabled:opacity-50"
          >
            Send all allocated ({allocatedReady.length})
          </button>
          <button
            type="button"
            onClick={runSweep}
            disabled={busy === "sweep"}
            className="px-4 py-2 border border-neutral-300 text-xs font-bold uppercase tracking-widest"
          >
            Run overdue sweep
          </button>
          <button
            type="button"
            onClick={logout}
            className="px-4 py-2 text-xs font-bold uppercase tracking-widest text-red-600"
          >
            Sign out
          </button>
        </div>
      </div>

      {error && (
        <div className="p-4 bg-red-50 border border-red-300 text-red-800 text-sm rounded">
          {error}
        </div>
      )}

      {groups.length === 0 && (
        <p className="text-neutral-600">No registrations match this filter.</p>
      )}

      <div className="flex flex-col gap-4">
        {groups.map((g) => {
          const fw = fullWeekCampers(g);
          const canAlloc =
            g.payment_status === "paid" &&
            g.billing_status !== "balance_paid" &&
            g.billing_status !== "invoiced";
          const canInvoice =
            g.billing_status === "allocated" && allFullWeekAllocated(g);
          const canRelease = g.billing_status === "invoiced";
          const overdue =
            g.billing_status === "invoiced" &&
            g.invoice_due_at &&
            new Date(g.invoice_due_at) < new Date();

          return (
            <article
              key={g.id}
              className={`bg-white border rounded-xl p-5 ${
                overdue ? "border-red-400" : "border-neutral-300"
              }`}
            >
              <div className="flex flex-wrap justify-between gap-3 mb-4">
                <div>
                  <p className="font-bold text-neutral-900">
                    {g.contact_first_name} {g.contact_last_name}
                  </p>
                  <p className="text-sm text-neutral-600">{g.contact_email}</p>
                  <p className="text-xs text-neutral-500 mt-1 font-mono">
                    {g.id}
                  </p>
                </div>
                <div className="text-right text-sm">
                  <p>
                    Deposit:{" "}
                    <span className="font-semibold">{g.payment_status}</span>
                  </p>
                  <p>
                    Balance:{" "}
                    <span className="font-semibold">
                      {billingLabel(g.billing_status)}
                    </span>
                  </p>
                  {g.invoice_due_at && (
                    <p className="text-xs text-neutral-500 mt-1">
                      Due: {formatDue(g.invoice_due_at)}
                      {overdue && (
                        <span className="text-red-600 font-bold ml-1">
                          OVERDUE
                        </span>
                      )}
                    </p>
                  )}
                </div>
              </div>

              {fw.length > 0 && (
                <div className="overflow-x-auto">
                  <table className="w-full text-sm border-collapse">
                    <thead>
                      <tr className="text-left text-xs uppercase tracking-widest text-neutral-500 border-b">
                        <th className="py-2 pr-4">Camper</th>
                        <th className="py-2 pr-4">Age</th>
                        <th className="py-2 pr-4">1st choice</th>
                        <th className="py-2">Allocated tier</th>
                      </tr>
                    </thead>
                    <tbody>
                      {fw.map((c) => {
                        return (
                          <tr key={c.id} className="border-b border-neutral-100">
                            <td className="py-2 pr-4">
                              {c.first_name} {c.last_name}
                            </td>
                            <td className="py-2 pr-4">{c.age}</td>
                            <td className="py-2 pr-4 text-neutral-600">
                              {c.accommodation_first_choice ?? "—"}
                            </td>
                            <td className="py-2">
                              <select
                                value={alloc[g.id]?.[c.id] ?? ""}
                                disabled={!canAlloc}
                                onChange={(e) =>
                                  setCamperAlloc(g.id, c.id, e.target.value)
                                }
                                className="w-full max-w-xs px-2 py-1 border border-neutral-300 text-sm"
                              >
                                <option value="">— Select —</option>
                                {accommodations.map((a) => {
                                  const disabled =
                                    a.code === ACCOMMODATION_CHILD_CODE &&
                                    c.age > 12;
                                  return (
                                    <option
                                      key={a.code}
                                      value={a.code}
                                      disabled={disabled}
                                    >
                                      {a.display_name}
                                      {a.stripe_price_id
                                        ? ""
                                        : " (no price id)"}
                                    </option>
                                  );
                                })}
                              </select>
                              {c.age < MIN_DEPOSIT_AGE &&
                                alloc[g.id]?.[c.id] ===
                                  ACCOMMODATION_CHILD_CODE && (
                                  <p className="text-xs text-neutral-500 mt-1">
                                    Under {MIN_DEPOSIT_AGE}: £0 balance price
                                    applies
                                  </p>
                                )}
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              )}

              {fw.length === 0 && (
                <p className="text-sm text-neutral-500 italic">
                  Day-pass only — no balance invoice.
                </p>
              )}

              <div className="flex flex-wrap gap-2 mt-4 pt-4 border-t border-neutral-200">
                {canAlloc && fw.length > 0 && (
                  <button
                    type="button"
                    disabled={busy === `alloc-${g.id}` || !allFullWeekAllocated(g)}
                    onClick={() => saveAllocation(g)}
                    className="px-4 py-2 bg-neutral-900 text-white text-xs font-bold uppercase tracking-widest disabled:opacity-50"
                  >
                    Save allocation
                  </button>
                )}
                {canInvoice && (
                  <button
                    type="button"
                    disabled={busy === `inv-${g.id}`}
                    onClick={() => sendInvoice(g)}
                    className="px-4 py-2 bg-primary text-white text-xs font-bold uppercase tracking-widest disabled:opacity-50"
                  >
                    Finalize &amp; send invoice
                  </button>
                )}
                {g.billing_status === "invoiced" && (
                  <>
                    <button
                      type="button"
                      disabled={busy === `res-${g.id}`}
                      onClick={() => resendInvoice(g)}
                      className="px-4 py-2 border border-neutral-300 text-xs font-bold uppercase tracking-widest"
                    >
                      Resend invoice email
                    </button>
                    <button
                      type="button"
                      disabled={busy === `ext-${g.id}`}
                      onClick={() => extendDue(g)}
                      className="px-4 py-2 border border-neutral-300 text-xs font-bold uppercase tracking-widest"
                    >
                      Extend due date
                    </button>
                  </>
                )}
                {canRelease && (
                  <button
                    type="button"
                    disabled={busy === `rel-${g.id}`}
                    onClick={() => releaseGroup(g)}
                    className="px-4 py-2 text-red-600 border border-red-300 text-xs font-bold uppercase tracking-widest"
                  >
                    Void &amp; release
                  </button>
                )}
              </div>
            </article>
          );
        })}
      </div>
    </div>
  );
}

"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import {
  adminApi,
  openAdminEventStream,
  type AdminEvent,
  AdminApiError,
} from "@/lib/admin-api";

function formatEventTime(iso: string): string {
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

export default function AdminActivityPage() {
  const router = useRouter();
  const [events, setEvents] = useState<AdminEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [reconnecting, setReconnecting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchEvents = useCallback(async () => {
    try {
      const res = await adminApi.listEvents({ limit: 100 });
      setEvents(res.events ?? []);
      setError(null);
    } catch (err) {
      if (err instanceof AdminApiError && err.status === 401) {
        router.replace("/admin/login");
        return;
      }
      setError("Could not load activity log.");
    } finally {
      setLoading(false);
    }
  }, [router]);

  useEffect(() => {
    adminApi
      .checkSession()
      .then(() => fetchEvents())
      .catch(() => router.replace("/admin/login"));
  }, [fetchEvents, router]);

  useEffect(() => {
    let debounce: ReturnType<typeof setTimeout> | null = null;
    const es = openAdminEventStream({
      onOpen: () => {
        setReconnecting(false);
        void fetchEvents();
      },
      onError: () => setReconnecting(true),
      onEvent: () => {
        if (debounce) clearTimeout(debounce);
        debounce = setTimeout(() => {
          void fetchEvents();
        }, 400);
      },
    });
    return () => {
      if (debounce) clearTimeout(debounce);
      es?.close();
    };
  }, [fetchEvents]);

  if (loading) {
    return (
      <div className="py-24 text-center text-neutral-500">Loading activity…</div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-extrabold text-neutral-800">
            Activity log
          </h1>
          <p className="text-sm text-neutral-600 mt-1">
            Who did what in the admin dashboard — updates live as your team
            works.
          </p>
          {reconnecting && (
            <p className="text-xs text-amber-700 mt-2">Reconnecting…</p>
          )}
        </div>
        <Link
          href="/admin"
          className="px-4 py-2 text-sm font-semibold text-neutral-700 bg-white border border-neutral-300 rounded-lg hover:bg-neutral-100"
        >
          Back to dashboard
        </Link>
      </div>

      {error && (
        <div className="p-3 bg-red-50 border border-red-300 text-red-800 text-sm rounded-lg">
          {error}
        </div>
      )}

      {events.length === 0 ? (
        <div className="bg-white border border-dashed border-neutral-300 rounded-xl py-16 text-center text-neutral-500">
          No activity yet.
        </div>
      ) : (
        <ul className="bg-white border border-neutral-300 rounded-xl divide-y divide-neutral-200">
          {events.map((ev) => (
            <li key={ev.id} className="p-4 sm:p-5 flex flex-col gap-1">
              <p className="text-sm text-neutral-800">{ev.summary}</p>
              <p className="text-xs text-neutral-500">
                <span className="font-semibold text-neutral-700">
                  {ev.actor_name}
                </span>
                {" · "}
                {formatEventTime(ev.created_at)}
              </p>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

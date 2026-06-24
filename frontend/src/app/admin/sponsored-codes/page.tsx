"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import {
  adminApi,
  type FreeCode,
  AdminApiError,
} from "@/lib/admin-api";

function formatTime(iso: string): string {
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

function codeStatus(c: FreeCode): string {
  if (c.used_at) return "Used";
  if (c.revoked_at) return "Revoked";
  return "Unused";
}

export default function AdminSponsoredCodesPage() {
  const router = useRouter();
  const [codes, setCodes] = useState<FreeCode[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [password, setPassword] = useState("");
  const [note, setNote] = useState("");
  const [generating, setGenerating] = useState(false);
  const [generatedCode, setGeneratedCode] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [revoking, setRevoking] = useState<string | null>(null);

  const fetchCodes = useCallback(async () => {
    try {
      const res = await adminApi.listFreeCodes();
      setCodes(res.codes ?? []);
      setError(null);
    } catch (err) {
      if (err instanceof AdminApiError && err.status === 401) {
        router.replace("/admin/login");
        return;
      }
      setError("Could not load sponsorship codes.");
    } finally {
      setLoading(false);
    }
  }, [router]);

  useEffect(() => {
    adminApi
      .checkSession()
      .then(() => fetchCodes())
      .catch(() => router.replace("/admin/login"));
  }, [fetchCodes, router]);

  async function handleGenerate(e: React.FormEvent) {
    e.preventDefault();
    setGenerating(true);
    setError(null);
    setGeneratedCode(null);
    try {
      const res = await adminApi.generateFreeCode(password, note);
      setGeneratedCode(res.code);
      setPassword("");
      setNote("");
      await fetchCodes();
    } catch (err) {
      if (err instanceof AdminApiError) {
        setError(err.detail.message);
      } else {
        setError("Could not generate code.");
      }
    } finally {
      setGenerating(false);
    }
  }

  async function handleRevoke(id: string) {
    if (!confirm("Revoke this unused code? It can no longer be redeemed.")) {
      return;
    }
    setRevoking(id);
    setError(null);
    try {
      await adminApi.revokeFreeCode(id);
      await fetchCodes();
    } catch (err) {
      if (err instanceof AdminApiError) {
        setError(err.detail.message);
      } else {
        setError("Could not revoke code.");
      }
    } finally {
      setRevoking(null);
    }
  }

  async function copyCode(code: string) {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setError("Could not copy to clipboard.");
    }
  }

  if (loading) {
    return (
      <div className="py-24 text-center text-neutral-500">Loading codes…</div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-extrabold text-neutral-800">
            Sponsorship codes
          </h1>
          <p className="text-sm text-neutral-600 mt-1">
            Generate one-time codes for church-sponsored registrations. Requires
            the separate code-generation password.
          </p>
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

      <form
        onSubmit={handleGenerate}
        className="bg-white border border-neutral-300 rounded-xl p-5 flex flex-col gap-4 max-w-lg"
      >
        <h2 className="text-sm font-bold text-neutral-800 uppercase tracking-wide">
          Generate new code
        </h2>
        <div>
          <label className="block text-xs font-semibold text-neutral-600 mb-1">
            Code-generation password
          </label>
          <input
            type="password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="w-full px-3 py-2 border border-neutral-300 rounded-lg text-sm"
            autoComplete="current-password"
          />
        </div>
        <div>
          <label className="block text-xs font-semibold text-neutral-600 mb-1">
            Note (optional)
          </label>
          <input
            type="text"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="e.g. First-time camper — John Smith"
            className="w-full px-3 py-2 border border-neutral-300 rounded-lg text-sm"
          />
        </div>
        <button
          type="submit"
          disabled={generating}
          className="self-start px-4 py-2 text-sm font-bold text-white bg-violet-600 rounded-lg hover:bg-violet-700 disabled:opacity-50"
        >
          {generating ? "Generating…" : "Generate code"}
        </button>
        {generatedCode && (
          <div className="p-3 bg-violet-50 border border-violet-200 rounded-lg flex flex-wrap items-center gap-2">
            <code className="text-lg font-bold text-violet-900">
              {generatedCode}
            </code>
            <button
              type="button"
              onClick={() => copyCode(generatedCode)}
              className="px-3 py-1 text-xs font-semibold bg-white border border-violet-300 rounded-lg hover:bg-violet-100"
            >
              {copied ? "Copied!" : "Copy"}
            </button>
          </div>
        )}
      </form>

      {codes.length === 0 ? (
        <div className="bg-white border border-dashed border-neutral-300 rounded-xl py-16 text-center text-neutral-500">
          No codes generated yet.
        </div>
      ) : (
        <ul className="bg-white border border-neutral-300 rounded-xl divide-y divide-neutral-200">
          {codes.map((c) => (
            <li
              key={c.id}
              className="p-4 sm:p-5 flex flex-wrap items-start justify-between gap-3"
            >
              <div>
                <p className="font-mono font-bold text-neutral-800">{c.code}</p>
                <p className="text-xs text-neutral-500 mt-1">
                  {c.created_by} · {formatTime(c.created_at)} ·{" "}
                  <span
                    className={
                      codeStatus(c) === "Unused"
                        ? "text-green-700 font-semibold"
                        : "text-neutral-600"
                    }
                  >
                    {codeStatus(c)}
                  </span>
                </p>
                {c.note && (
                  <p className="text-sm text-neutral-600 mt-1">{c.note}</p>
                )}
              </div>
              {codeStatus(c) === "Unused" && (
                <button
                  type="button"
                  disabled={revoking === c.id}
                  onClick={() => handleRevoke(c.id)}
                  className="px-3 py-1.5 text-xs font-semibold text-red-700 bg-white border border-red-200 rounded-lg hover:bg-red-50 disabled:opacity-50"
                >
                  {revoking === c.id ? "Revoking…" : "Revoke"}
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

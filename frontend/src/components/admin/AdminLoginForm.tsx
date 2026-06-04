"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { adminApi, AdminApiError } from "@/lib/admin-api";

export default function AdminLoginForm() {
  const router = useRouter();
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      await adminApi.login(password);
      router.push("/admin");
      router.refresh();
    } catch (err) {
      if (err instanceof AdminApiError) {
        setError(err.detail.message);
      } else {
        setError("Login failed. Check the API URL and try again.");
      }
    } finally {
      setLoading(false);
    }
  }

  return (
    <form
      onSubmit={onSubmit}
      className="max-w-md mx-auto bg-white border border-neutral-300 rounded-xl p-8 flex flex-col gap-5"
    >
      <p className="text-xs font-bold uppercase tracking-widest text-primary">
        White Team
      </p>
      <h1 className="text-2xl font-extrabold text-neutral-900">
        Admin sign in
      </h1>
      <p className="text-sm text-neutral-600">
        Enter the shared team password to manage camp registrations and send
        balance invoices.
      </p>
      {error && (
        <div className="p-3 bg-red-50 border border-red-300 text-red-800 text-sm rounded">
          {error}
        </div>
      )}
      <div className="flex flex-col gap-2">
        <label
          htmlFor="admin-password"
          className="text-xs font-bold uppercase tracking-widest text-neutral-700"
        >
          Password
        </label>
        <input
          id="admin-password"
          type="password"
          value={password}
          required
          autoComplete="current-password"
          onChange={(e) => setPassword(e.target.value)}
          className="px-4 py-3 border border-neutral-300 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
        />
      </div>
      <button
        type="submit"
        disabled={loading}
        className="px-8 py-3 bg-primary text-white font-bold uppercase tracking-widest text-sm hover:bg-primary-dark disabled:opacity-60"
      >
        {loading ? "Signing in…" : "Sign in"}
      </button>
    </form>
  );
}

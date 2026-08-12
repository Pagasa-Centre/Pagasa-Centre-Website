"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { campAdminApi, CampAdminApiError } from "@/lib/camp-admin-api";

export default function CampAdminLoginForm() {
  const router = useRouter();
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      await campAdminApi.login(password, firstName.trim(), lastName.trim());
      router.push("/camp-admin");
      router.refresh();
    } catch (err) {
      if (err instanceof CampAdminApiError) {
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
      className="w-full max-w-md bg-white border border-neutral-300 rounded-2xl p-8 flex flex-col gap-5 shadow-sm"
    >
      <div>
        <h1 className="text-2xl font-extrabold text-neutral-800">
          Welcome, White Team
        </h1>
        <p className="text-sm text-neutral-600 mt-2">
          Enter your name and the shared team password to manage camp
          registrations.
        </p>
      </div>
      {error && (
        <div className="p-3 bg-red-50 border border-red-300 text-red-800 text-sm rounded-lg">
          {error}
        </div>
      )}
      <div className="grid grid-cols-2 gap-3">
        <div className="flex flex-col gap-2">
          <label
            htmlFor="admin-first-name"
            className="text-sm font-semibold text-neutral-700"
          >
            First name
          </label>
          <input
            id="admin-first-name"
            type="text"
            value={firstName}
            required
            autoFocus
            autoComplete="given-name"
            onChange={(e) => setFirstName(e.target.value)}
            className="px-4 py-3 border border-neutral-300 rounded-lg text-base focus:outline-none focus:ring-2 focus:ring-primary"
          />
        </div>
        <div className="flex flex-col gap-2">
          <label
            htmlFor="admin-last-name"
            className="text-sm font-semibold text-neutral-700"
          >
            Last name
          </label>
          <input
            id="admin-last-name"
            type="text"
            value={lastName}
            required
            autoComplete="family-name"
            onChange={(e) => setLastName(e.target.value)}
            className="px-4 py-3 border border-neutral-300 rounded-lg text-base focus:outline-none focus:ring-2 focus:ring-primary"
          />
        </div>
      </div>
      <div className="flex flex-col gap-2">
        <label
          htmlFor="admin-password"
          className="text-sm font-semibold text-neutral-700"
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
          className="px-4 py-3 border border-neutral-300 rounded-lg text-base focus:outline-none focus:ring-2 focus:ring-primary"
        />
      </div>
      <button
        type="submit"
        disabled={loading}
        className="px-8 py-3 bg-primary text-white font-bold rounded-lg text-base hover:bg-primary-dark disabled:opacity-60"
      >
        {loading ? "Signing in…" : "Sign in"}
      </button>
    </form>
  );
}

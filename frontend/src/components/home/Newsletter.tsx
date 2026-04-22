"use client";

import { useState } from "react";
import Image from "next/image";

export default function Newsletter() {
  const [form, setForm] = useState({ firstName: "", surname: "", email: "" });
  const [status, setStatus] = useState<"idle" | "loading" | "success" | "error">("idle");

  function handleChange(e: React.ChangeEvent<HTMLInputElement>) {
    setForm((f) => ({ ...f, [e.target.name]: e.target.value }));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!form.email) return;
    setStatus("loading");
    // TODO: wire up to Go API via newsletter.subscribe() once backend is live
    await new Promise((r) => setTimeout(r, 800));
    setStatus("success");
    setForm({ firstName: "", surname: "", email: "" });
  }

  return (
    <section className="bg-ink text-white">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-16">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-10 items-start">
          {/* Left — logo + tagline */}
          <div className="flex flex-col gap-6">
            <Image
              src="/pagasa-logo.png"
              alt="Pag-Asa Centre"
              width={100}
              height={100}
              className="object-contain"
            />
            <p className="text-white/70 text-base max-w-xs leading-relaxed">
              Subscribe for monthly updates about our upcoming church events
              and more!
            </p>
          </div>

          {/* Right — form */}
          {status === "success" ? (
            <div className="flex items-center">
              <p className="text-primary font-semibold text-lg">
                Thank you for subscribing! You&apos;ll hear from us soon.
              </p>
            </div>
          ) : (
            <form onSubmit={handleSubmit} className="flex flex-col gap-4">
              {/* Row 1: First name + Surname */}
              <div className="grid grid-cols-2 gap-4">
                <input
                  type="text"
                  name="firstName"
                  placeholder="First name"
                  value={form.firstName}
                  onChange={handleChange}
                  className="px-4 py-3 bg-white text-neutral-900 placeholder-neutral-500 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                />
                <input
                  type="text"
                  name="surname"
                  placeholder="Surname"
                  value={form.surname}
                  onChange={handleChange}
                  className="px-4 py-3 bg-white text-neutral-900 placeholder-neutral-500 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                />
              </div>
              {/* Row 2: Email + Subscribe */}
              <div className="grid grid-cols-2 gap-4">
                <input
                  type="email"
                  name="email"
                  required
                  placeholder="Email"
                  value={form.email}
                  onChange={handleChange}
                  className="px-4 py-3 bg-white text-neutral-900 placeholder-neutral-500 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                />
                <button
                  type="submit"
                  disabled={status === "loading"}
                  className="px-6 py-3 bg-primary text-white font-bold text-xs uppercase tracking-widest hover:bg-primary-dark transition-colors disabled:opacity-60"
                >
                  {status === "loading" ? "..." : "Subscribe"}
                </button>
              </div>
            </form>
          )}
        </div>
      </div>
    </section>
  );
}

"use client";

import { Suspense, useEffect, useMemo, useSyncExternalStore } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import type { SuccessStash } from "@/components/camp/CampRegisterForm";

const STASH_KEY = "pc-camp-last-registration";

// sessionStorage isn't reactive; subscribe is a no-op. Reading on the client
// returns the raw JSON; on the server it returns null. useSyncExternalStore
// gives us the right SSR/CSR semantics without scheduling extra renders.
function subscribe() {
  return () => {};
}
function getClientSnapshot(): string | null {
  try {
    return sessionStorage.getItem(STASH_KEY);
  } catch {
    return null;
  }
}
function getServerSnapshot(): string | null {
  return null;
}

type Props = {
  apiBase: string;
};

function SuccessBody({ apiBase }: Props) {
  const sp = useSearchParams();
  const sessionId = sp.get("session_id");
  const stashJson = useSyncExternalStore(
    subscribe,
    getClientSnapshot,
    getServerSnapshot,
  );
  const stash = useMemo<SuccessStash | null>(() => {
    if (!stashJson) return null;
    try {
      return JSON.parse(stashJson) as SuccessStash;
    } catch {
      return null;
    }
  }, [stashJson]);

  // Compose the absolute consent URL. Backend may have returned a relative
  // path or none at all; fall back to apiBase + /api/consent-form.
  const consentURL = stash?.has_minor
    ? stash.consent_form_url && stash.consent_form_url.startsWith("http")
      ? stash.consent_form_url
      : `${apiBase}/api/consent-form`
    : null;

  useEffect(() => {
    if (!stash?.has_minor || !consentURL) return;
    const a = document.createElement("a");
    a.href = consentURL;
    a.download = "pc-summer-camp-2026-parental-consent.pdf";
    document.body.appendChild(a);
    a.click();
    a.remove();
  }, [stash, consentURL]);

  return (
    <section className="bg-surface py-20 lg:py-28">
      <div className="max-w-2xl mx-auto px-4 sm:px-6 lg:px-8 text-center">
        <div className="w-14 h-1 bg-primary mb-7 mx-auto" />
        <p className="text-primary uppercase tracking-widest text-sm font-semibold mb-3">
          Registration received
        </p>
        <h1 className="text-3xl sm:text-4xl font-extrabold text-neutral-900 leading-tight mb-5">
          You&apos;re registered for camp
        </h1>
        <p className="text-neutral-600 mb-3">
          Thank you for signing up to PC Summer Camp 2026. Your payment was
          processed by Stripe and a receipt will arrive in your inbox shortly.
        </p>
        {sessionId && (
          <p className="text-xs text-neutral-400 mb-8">
            Reference: <span className="font-mono">{sessionId}</span>
          </p>
        )}

        {stash?.has_minor && consentURL && (
          <div className="p-5 bg-primary/10 border border-primary/30 text-left text-sm text-neutral-800 mb-8">
            <p className="font-bold mb-2">Parental Consent Form</p>
            <p className="mb-3">
              At least one registered camper is under 18. The Parental Consent
              Form should have downloaded automatically. If it didn&apos;t, use
              the link below:
            </p>
            <a
              href={consentURL}
              download
              className="inline-flex items-center px-5 py-2.5 bg-primary text-white text-xs uppercase tracking-widest font-bold hover:bg-primary-dark transition-colors"
            >
              Download Parental Consent Form
            </a>
          </div>
        )}

        <p className="text-neutral-600 mb-8">
          More information will be announced soon. For now, please reach out
          to your cell or network leader if you have any questions. God bless!
        </p>

        <div className="flex flex-wrap gap-3 justify-center">
          <Link
            href="/"
            className="px-8 py-3 bg-neutral-900 text-white font-bold uppercase tracking-widest text-sm hover:bg-primary transition-colors"
          >
            Back to home
          </Link>
          <Link
            href="/events"
            className="px-8 py-3 bg-white text-neutral-900 border border-neutral-300 font-bold uppercase tracking-widest text-sm hover:border-primary hover:text-primary transition-colors"
          >
            See more events
          </Link>
        </div>
      </div>
    </section>
  );
}

export default function RegistrationSuccess({ apiBase }: Props) {
  // useSearchParams requires a Suspense boundary in Next.js app router.
  return (
    <Suspense
      fallback={
        <section className="bg-surface py-20 lg:py-28 text-center">
          <p className="text-neutral-600">Loading…</p>
        </section>
      }
    >
      <SuccessBody apiBase={apiBase} />
    </Suspense>
  );
}

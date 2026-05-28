"use client";

import {
  Suspense,
  useEffect,
  useMemo,
  useState,
  useSyncExternalStore,
} from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import { camp, type SummaryCamper } from "@/lib/api";
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
  const groupId = sp.get("group_id");
  // ?free=1 is set by the form when the total is £0 (day-pass-only) — no
  // Stripe round-trip happens in that case, so the messaging needs to be
  // tweaked to not promise a Stripe receipt that won't arrive.
  const isFreeRegistration = sp.get("free") === "1";
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

  // Camper list: prefer the client-side stash (fresh from the just-submitted
  // form) because it doesn't need a backend round-trip. If the stash is
  // missing (different tab / cleared session) and we have an identifier,
  // fall back to /api/registrations/summary.
  const [remoteCampers, setRemoteCampers] = useState<SummaryCamper[] | null>(
    null,
  );
  const [remoteContactEmail, setRemoteContactEmail] = useState<string | null>(
    null,
  );
  useEffect(() => {
    if (stash?.campers && stash.campers.length > 0) return;
    if (!sessionId && !groupId) return;
    let cancelled = false;
    camp
      .summary({
        sessionId: sessionId ?? undefined,
        groupId: groupId ?? undefined,
      })
      .then((res) => {
        if (cancelled) return;
        setRemoteCampers(res.campers);
        setRemoteContactEmail(res.contact_email);
      })
      .catch(() => {
        // Silent — we'll just not show the list if the lookup fails.
      });
    return () => {
      cancelled = true;
    };
  }, [stash, sessionId, groupId]);

  const campers: { first_name: string; last_name: string }[] =
    stash?.campers && stash.campers.length > 0
      ? stash.campers
      : (remoteCampers ?? []);
  const contactEmail =
    stash?.contact_email ?? remoteContactEmail ?? null;

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
      <div className="max-w-2xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="w-14 h-1 bg-primary mb-7 mx-auto" />
        <p className="text-primary uppercase tracking-widest text-sm font-semibold mb-3 text-center">
          {isFreeRegistration ? "Registration confirmed" : "Deposit received"}
        </p>
        <h1 className="text-3xl sm:text-4xl font-extrabold text-neutral-900 leading-tight mb-5 text-center">
          {isFreeRegistration
            ? "You're registered for camp"
            : "Thanks, your deposit is in"}
        </h1>
        <p className="text-neutral-700 mb-3 text-center">
          {isFreeRegistration
            ? "Thank you for registering for PC Summer Camp 2026. Day-pass attendance doesn't require a deposit — we'll see you on the day."
            : "Thank you for signing up to PC Summer Camp 2026. Your non-refundable deposit has been received. A separate payment receipt will arrive from Stripe shortly."}
        </p>
        <p className="text-neutral-700 mb-3 text-center">
          A confirmation email has been sent to{" "}
          <span className="font-semibold">
            {contactEmail ?? "the email you provided"}
          </span>
          . Check your spam folder if you don&apos;t see it within a few
          minutes.
        </p>
        {sessionId && (
          <p className="text-xs text-neutral-400 mb-8 text-center">
            Stripe reference: <span className="font-mono">{sessionId}</span>
          </p>
        )}

        {/* Registered campers — public-safe summary so users can screenshot
            confirmation. Names only; no PII beyond what they just typed. */}
        {campers.length > 0 && (
          <div className="bg-white border border-neutral-300 rounded-xl p-6 mb-8">
            <p className="text-xs font-bold uppercase tracking-widest text-neutral-500 mb-3">
              Registered campers
            </p>
            <p className="text-sm text-neutral-700 mb-4">
              We can confirm that the following camper
              {campers.length === 1 ? " has" : "s have"} been registered for PC
              Summer Camp 2026. Feel free to screenshot this page for your
              records.
            </p>
            <ul className="flex flex-col gap-1.5 text-sm text-neutral-900">
              {campers.map((c, i) => (
                <li key={`${c.first_name}-${c.last_name}-${i}`} className="font-medium">
                  • {c.first_name} {c.last_name}
                </li>
              ))}
            </ul>
          </div>
        )}

        {/* Key dates timeline */}
        <div className="bg-white border border-neutral-300 rounded-xl p-6 mb-8">
          <p className="text-xs font-bold uppercase tracking-widest text-neutral-500 mb-4">
            What happens next
          </p>
          <ol className="flex flex-col gap-4">
            <li className="flex gap-4">
              <span className="flex-shrink-0 w-8 h-8 bg-primary/10 text-primary flex items-center justify-center font-bold text-sm">
                1
              </span>
              <div>
                <p className="font-semibold text-neutral-900 text-sm">
                  1 – 30 June: Registration window
                </p>
                <p className="text-sm text-neutral-600">
                  Tell friends and family before registrations close.
                </p>
              </div>
            </li>
            <li className="flex gap-4">
              <span className="flex-shrink-0 w-8 h-8 bg-primary/10 text-primary flex items-center justify-center font-bold text-sm">
                2
              </span>
              <div>
                <p className="font-semibold text-neutral-900 text-sm">
                  1 – 15 July: Room allocation
                </p>
                <p className="text-sm text-neutral-600">
                  The White Team allocates rooms and posts the temporary
                  allocation so you know the balance for final payment.
                </p>
              </div>
            </li>
            <li className="flex gap-4">
              <span className="flex-shrink-0 w-8 h-8 bg-primary/10 text-primary flex items-center justify-center font-bold text-sm">
                3
              </span>
              <div>
                <p className="font-semibold text-neutral-900 text-sm">
                  16 – 31 July: Final payment
                </p>
                <p className="text-sm text-neutral-600">
                  Settle your balance to lock in your room. Once fully paid,
                  you&apos;ll get your final accommodation confirmation by
                  email, in person, or via your cell leader.
                </p>
              </div>
            </li>
          </ol>
        </div>

        {stash?.has_minor && consentURL && (
          <div className="p-5 bg-primary/10 border border-primary/30 text-left text-sm text-neutral-800 mb-8 rounded">
            <p className="font-bold mb-2">Parental Consent Form</p>
            <p className="mb-3">
              At least one registered camper is under 18. The Parental Consent
              Form should have downloaded automatically — if not, use the
              button below. <strong>Print it, sign it in ink, and hand
              the completed form to Bro Ash</strong> before the start of camp.
              We don&apos;t accept emailed scans.
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

        <p className="text-neutral-600 mb-8 text-center">
          Got questions? Speak to your cell leader. God bless!
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

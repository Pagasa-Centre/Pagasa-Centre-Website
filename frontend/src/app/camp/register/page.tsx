import type { Metadata } from "next";
import Navbar from "@/components/layout/Navbar";
import Footer from "@/components/layout/Footer";
import CampRegisterHero from "@/components/camp/CampRegisterHero";
import CampRegisterForm from "@/components/camp/CampRegisterForm";
import RegistrationsClosedNotice from "@/components/camp/RegistrationsClosedNotice";
import { camp } from "@/lib/api";

export const metadata: Metadata = {
  title: "Register | PC Summer Camp 2026 | Pagasa Centre",
  description:
    "Register for Pag-Asa Centre Summer Camp 2026, 10–14 August at Lenchwood Trust, Evesham.",
};

export const dynamic = "force-dynamic";

export default async function CampRegisterPage() {
  const [config, pricesRes, accommodationsRes, shirtSizesRes, pricing] =
    await Promise.all([
      camp.config(),
      camp.prices(),
      camp.accommodations(),
      camp.shirtSizes(),
      camp.registrationPricing().catch(() => null),
    ]);

  const isFullMode =
    config.registration_payment_mode === "full" ||
    pricing?.mode === "full";

  return (
    <>
      <Navbar />
      <main>
        <CampRegisterHero config={config} />
        {!config.registrations_open ? (
          <RegistrationsClosedNotice />
        ) : isFullMode && !pricing ? (
          <section className="bg-surface py-16 lg:py-20">
            <div className="max-w-2xl mx-auto px-4 sm:px-6 lg:px-8 text-center">
              <div className="bg-amber-50 border border-amber-300 rounded-xl p-8">
                <p className="text-lg font-bold text-neutral-900 mb-2">
                  Registration temporarily unavailable
                </p>
                <p className="text-sm text-neutral-700">
                  We couldn&apos;t load camp pricing right now. Please try again
                  in a few minutes or contact the White Team.
                </p>
              </div>
            </div>
          </section>
        ) : (
          <CampRegisterForm
            config={config}
            prices={pricesRes.prices}
            pricing={pricing}
            accommodations={accommodationsRes.accommodations}
            shirtSizes={shirtSizesRes.sizes}
          />
        )}
      </main>
      <Footer />
    </>
  );
}

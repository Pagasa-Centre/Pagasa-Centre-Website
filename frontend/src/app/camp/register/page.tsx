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
  const [config, pricesRes, accommodationsRes, shirtSizesRes] =
    await Promise.all([
      camp.config(),
      camp.prices(),
      camp.accommodations(),
      camp.shirtSizes(),
    ]);

  return (
    <>
      <Navbar />
      <main>
        <CampRegisterHero config={config} />
        {!config.registrations_open ? (
          <RegistrationsClosedNotice />
        ) : (
          <CampRegisterForm
            config={config}
            prices={pricesRes.prices}
            accommodations={accommodationsRes.accommodations}
            shirtSizes={shirtSizesRes.sizes}
          />
        )}
      </main>
      <Footer />
    </>
  );
}

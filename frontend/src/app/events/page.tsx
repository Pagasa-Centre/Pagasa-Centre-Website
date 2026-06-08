import type { Metadata } from "next";
import Navbar from "@/components/layout/Navbar";
import Footer from "@/components/layout/Footer";
import Newsletter from "@/components/home/Newsletter";
import EventsHero from "@/components/events/EventsHero";
import EventsGrid from "@/components/events/EventsGrid";
import { camp } from "@/lib/api";

export const metadata: Metadata = {
  title: "Events - Pagasa Centre",
  description:
    "Stay up to date with Pagasa Centre's upcoming church events designed to encourage faith and fellowship.",
};

// Read the live camp config on every request so the Summer Camp CTA reflects
// the current open/closed state the moment the White Team toggles it.
export const dynamic = "force-dynamic";

export default async function EventsPage() {
  // Fail open: if the config can't be read, show the normal Register CTA.
  let registrationsOpen = true;
  try {
    const config = await camp.config();
    registrationsOpen = config.registrations_open;
  } catch {
    registrationsOpen = true;
  }

  return (
    <>
      <Navbar />
      <main>
        <EventsHero />
        <EventsGrid registrationsOpen={registrationsOpen} />
        <Newsletter />
      </main>
      <Footer />
    </>
  );
}

import type { Metadata } from "next";
import Navbar from "@/components/layout/Navbar";
import Footer from "@/components/layout/Footer";
import Newsletter from "@/components/home/Newsletter";
import EventsHero from "@/components/events/EventsHero";
import EventsGrid from "@/components/events/EventsGrid";

export const metadata: Metadata = {
  title: "Events - Pagasa Centre",
  description:
    "Stay up to date with Pagasa Centre's upcoming church events designed to encourage faith and fellowship.",
};

export default function EventsPage() {
  return (
    <>
      <Navbar />
      <main>
        <EventsHero />
        <EventsGrid />
        <Newsletter />
      </main>
      <Footer />
    </>
  );
}

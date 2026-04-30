import type { Metadata } from "next";
import Navbar from "@/components/layout/Navbar";
import Footer from "@/components/layout/Footer";
import Newsletter from "@/components/home/Newsletter";
import EventsHero from "@/components/events/EventsHero";
import EventsGrid from "@/components/events/EventsGrid";

export const metadata: Metadata = {
  title: "Events | Pag-Asa Centre",
  description:
    "Stay connected with upcoming Pag-Asa Centre events — conferences, anniversaries, summer camp, and more.",
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

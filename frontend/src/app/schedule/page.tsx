import type { Metadata } from "next";
import Navbar from "@/components/layout/Navbar";
import Footer from "@/components/layout/Footer";
import Newsletter from "@/components/home/Newsletter";
import ScheduleHero from "@/components/schedule/ScheduleHero";
import WeeklyServices from "@/components/schedule/WeeklyServices";
import MinistriesGrid from "@/components/schedule/MinistriesGrid";
import GreatCommissionCTA from "@/components/schedule/GreatCommissionCTA";

export const metadata: Metadata = {
  title: "Schedule | Pag-Asa Centre",
  description:
    "Pag-Asa Centre weekly services and ministry schedule. Find a place to belong, grow, and serve.",
};

export default function SchedulePage() {
  return (
    <>
      <Navbar />
      <main>
        <ScheduleHero />
        <WeeklyServices />
        <MinistriesGrid />
        <GreatCommissionCTA />
        <Newsletter />
      </main>
      <Footer />
    </>
  );
}

import type { Metadata } from "next";
import Navbar from "@/components/layout/Navbar";
import Footer from "@/components/layout/Footer";
import Newsletter from "@/components/home/Newsletter";
import ScheduleHero from "@/components/schedule/ScheduleHero";
import WeeklyServices from "@/components/schedule/WeeklyServices";
import MinistriesGrid from "@/components/schedule/MinistriesGrid";
import GreatCommissionCTA from "@/components/schedule/GreatCommissionCTA";

export const metadata: Metadata = {
  title: "Schedule - Pagasa Centre",
  description: "Check out our schedule!",
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

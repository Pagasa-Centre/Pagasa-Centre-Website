import type { Metadata } from "next";
import Navbar from "@/components/layout/Navbar";
import Footer from "@/components/layout/Footer";
import AboutHero from "@/components/about/AboutHero";
import OurStory from "@/components/about/OurStory";
import G12Vision from "@/components/about/G12Vision";
import AboutLeadership from "@/components/about/AboutLeadership";
import StatementOfFaith from "@/components/about/StatementOfFaith";

export const metadata: Metadata = {
  title: "About Us | Pag-Asa Centre",
  description:
    "Learn about Pag-Asa Centre — a non-denominational UK charity church established in 2007, our G12 vision, leadership, and statement of faith.",
};

export default function AboutPage() {
  return (
    <>
      <Navbar />
      <main className="pt-[88px]">
        <AboutHero />
        <OurStory />
        <G12Vision />
        <AboutLeadership />
        <StatementOfFaith />
      </main>
      <Footer />
    </>
  );
}

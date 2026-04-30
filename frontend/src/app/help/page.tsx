import type { Metadata } from "next";
import Navbar from "@/components/layout/Navbar";
import Footer from "@/components/layout/Footer";
import Newsletter from "@/components/home/Newsletter";
import HelpHero from "@/components/help/HelpHero";
import CharityStory from "@/components/help/CharityStory";
import HelpContactCTA from "@/components/help/HelpContactCTA";
import AboutLocations from "@/components/about/AboutLocations";

export const metadata: Metadata = {
  title: "How Can I Help | Pag-Asa Centre",
  description:
    "Help us build a permanent home for Pag-Asa Centre. Support our church building project, share a fundraising idea, or join us in serving local and global communities.",
};

export default function HelpPage() {
  return (
    <>
      <Navbar />
      <main>
        <HelpHero />
        <CharityStory />
        <HelpContactCTA />
        <AboutLocations />
        <Newsletter />
      </main>
      <Footer />
    </>
  );
}

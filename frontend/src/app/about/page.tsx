import type { Metadata } from "next";
import Navbar from "@/components/layout/Navbar";
import Footer from "@/components/layout/Footer";
import Newsletter from "@/components/home/Newsletter";
import AboutHero from "@/components/about/AboutHero";
import AboutVideo from "@/components/about/AboutVideo";
import OurStory from "@/components/about/OurStory";
import G12Vision from "@/components/about/G12Vision";
import Pastors from "@/components/about/Pastors";
import StatementOfFaith from "@/components/about/StatementOfFaith";
import AboutLocations from "@/components/about/AboutLocations";

export const metadata: Metadata = {
  title: "About Us | Pag-Asa Centre",
  description:
    "Pag-Asa Centre is a non-denominational, UK-registered charity church established in 2007. Learn about our story, the G12 vision, our pastors, and our statement of faith.",
};

export default function AboutPage() {
  return (
    <>
      <Navbar />
      <main>
        <AboutHero />
        <AboutVideo />
        <OurStory />
        <G12Vision />
        <Pastors />
        <StatementOfFaith />
        <AboutLocations />
        <Newsletter />
      </main>
      <Footer />
    </>
  );
}

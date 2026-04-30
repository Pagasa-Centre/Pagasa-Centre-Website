import type { Metadata } from "next";
import Navbar from "@/components/layout/Navbar";
import Footer from "@/components/layout/Footer";
import Newsletter from "@/components/home/Newsletter";
import ContactHero from "@/components/contact/ContactHero";
import ContactForm from "@/components/contact/ContactForm";
import ContactFAQ from "@/components/contact/ContactFAQ";
import AboutLocations from "@/components/about/AboutLocations";

export const metadata: Metadata = {
  title: "Contact Us | Pag-Asa Centre",
  description:
    "Reach out to Pag-Asa Centre for questions, prayer requests, or to learn more about our church family. We're here to help.",
};

export default function ContactPage() {
  return (
    <>
      <Navbar />
      <main>
        <ContactHero />
        <ContactForm />
        <AboutLocations />
        <ContactFAQ />
        <Newsletter />
      </main>
      <Footer />
    </>
  );
}

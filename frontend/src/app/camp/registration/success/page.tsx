import type { Metadata } from "next";
import Navbar from "@/components/layout/Navbar";
import Footer from "@/components/layout/Footer";
import RegistrationSuccess from "@/components/camp/RegistrationSuccess";

export const metadata: Metadata = {
  title: "Registration confirmed | PC Summer Camp 2026",
  description: "Your Summer Camp 2026 registration has been received.",
};

// The API base needs to be visible to the client so it can build the consent
// PDF download URL. NEXT_PUBLIC_* is inlined at build time.
const API_BASE =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export default function CampSuccessPage() {
  return (
    <>
      <Navbar />
      <main className="pt-[88px]">
        <RegistrationSuccess apiBase={API_BASE} />
      </main>
      <Footer />
    </>
  );
}

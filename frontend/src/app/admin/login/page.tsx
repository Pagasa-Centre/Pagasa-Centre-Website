import type { Metadata } from "next";
import Navbar from "@/components/layout/Navbar";
import Footer from "@/components/layout/Footer";
import AdminLoginForm from "@/components/admin/AdminLoginForm";

export const metadata: Metadata = {
  title: "Admin sign in | Pagasa Centre",
  robots: { index: false, follow: false },
};

export default function AdminLoginPage() {
  return (
    <>
      <Navbar />
      <main className="bg-surface py-16 lg:py-24 min-h-[60vh]">
        <AdminLoginForm />
      </main>
      <Footer />
    </>
  );
}

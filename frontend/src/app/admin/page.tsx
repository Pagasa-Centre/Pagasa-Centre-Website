import type { Metadata } from "next";
import Navbar from "@/components/layout/Navbar";
import Footer from "@/components/layout/Footer";
import AdminDashboard from "@/components/admin/AdminDashboard";

export const metadata: Metadata = {
  title: "Camp admin | Pagasa Centre",
  robots: { index: false, follow: false },
};

export default function AdminPage() {
  return (
    <>
      <Navbar />
      <main className="bg-surface py-10 lg:py-14">
        <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8">
          <AdminDashboard />
        </div>
      </main>
      <Footer />
    </>
  );
}

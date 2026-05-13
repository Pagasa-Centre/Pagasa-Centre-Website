import type { Metadata } from "next";
import Link from "next/link";
import Navbar from "@/components/layout/Navbar";
import Footer from "@/components/layout/Footer";

export const metadata: Metadata = {
  title: "Payment cancelled | PC Summer Camp 2026",
  description: "Your camp payment was cancelled before completion.",
};

export default function CampCancelledPage() {
  return (
    <>
      <Navbar />
      <main className="pt-[88px]">
        <section className="bg-surface py-20 lg:py-28">
          <div className="max-w-2xl mx-auto px-4 sm:px-6 lg:px-8 text-center">
            <div className="w-14 h-1 bg-neutral-800 mb-7 mx-auto" />
            <p className="text-primary uppercase tracking-widest text-sm font-semibold mb-3">
              Payment cancelled
            </p>
            <h1 className="text-3xl sm:text-4xl font-extrabold text-neutral-900 leading-tight mb-5">
              No worries — you weren&apos;t charged
            </h1>
            <p className="text-neutral-600 mb-8">
              Your payment was cancelled before it completed, so your
              registration hasn&apos;t been confirmed yet. You can head back
              and try again whenever you&apos;re ready.
            </p>
            <div className="flex flex-wrap gap-3 justify-center">
              <Link
                href="/camp/register"
                className="px-8 py-3 bg-primary text-white font-bold uppercase tracking-widest text-sm hover:bg-primary-dark transition-colors"
              >
                Try again
              </Link>
              <Link
                href="/contact"
                className="px-8 py-3 bg-white text-neutral-900 border border-neutral-300 font-bold uppercase tracking-widest text-sm hover:border-primary hover:text-primary transition-colors"
              >
                Need help?
              </Link>
            </div>
          </div>
        </section>
      </main>
      <Footer />
    </>
  );
}

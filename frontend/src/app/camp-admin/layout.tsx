import type { Metadata } from "next";
import Image from "next/image";

export const metadata: Metadata = {
  title: "Summer Camp Admin | Pagasa Centre",
  robots: { index: false, follow: false },
};

// Dedicated, distraction-free shell for the White Team. Intentionally does NOT
// include the public site Navbar/Footer so there is maximum room for the
// allocation + invoicing workflow.
export default function CampAdminLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <div className="min-h-screen flex flex-col bg-neutral-200">
      <header className="bg-white border-b border-neutral-300">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 py-3 flex items-center gap-3">
          <Image
            src="/pagasa-logo.png"
            alt="Pagasa Centre"
            width={36}
            height={36}
            className="rounded-full"
          />
          <div className="leading-tight">
            <p className="text-sm font-extrabold text-neutral-800">
              Summer Camp 2026
            </p>
            <p className="text-xs text-neutral-500">White Team admin</p>
          </div>
        </div>
      </header>
      <main className="flex-1 w-full">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 py-6 lg:py-8">
          {children}
        </div>
      </main>
    </div>
  );
}

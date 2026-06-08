import Image from "next/image";
import type { CampConfig } from "@/lib/api";
import { CAMP_REGISTRATION_CLOSED_LABEL } from "@/lib/events";

function formatDateRange(startISO: string, endISO: string): string {
  try {
    const start = new Date(startISO);
    const end = new Date(endISO);
    const sameMonth = start.getUTCMonth() === end.getUTCMonth();
    const startDay = start.getUTCDate();
    const endDay = end.getUTCDate();
    const monthShort = (d: Date) =>
      d.toLocaleString("en-GB", { month: "long", timeZone: "UTC" });
    const year = end.getUTCFullYear();
    return sameMonth
      ? `${startDay}–${endDay} ${monthShort(end)} ${year}`
      : `${startDay} ${monthShort(start)} – ${endDay} ${monthShort(end)} ${year}`;
  } catch {
    return `${startISO} – ${endISO}`;
  }
}

export default function CampRegisterHero({ config }: { config: CampConfig }) {
  return (
    <section className="relative min-h-[480px] flex items-center justify-center overflow-hidden pt-[88px]">
      <Image
        src="/hero-bg.jpg"
        alt="Pag-Asa Centre summer camp"
        fill
        priority
        className="object-cover object-center"
      />
      <div className="absolute inset-0 bg-black/65" />
      <div className="relative z-10 text-center text-white px-4 sm:px-6 max-w-3xl mx-auto py-16 lg:py-20">
        <p className="text-primary-light uppercase tracking-widest text-sm font-semibold mb-4">
          {config.registrations_open
            ? "Register Now"
            : CAMP_REGISTRATION_CLOSED_LABEL}
        </p>
        <h1 className="text-4xl sm:text-5xl lg:text-6xl font-extrabold leading-tight mb-5 tracking-tight">
          {config.name}
        </h1>
        <p className="text-white/85 text-lg sm:text-xl mb-3 leading-relaxed">
          {formatDateRange(config.start_date, config.end_date)} ·{" "}
          {config.location_name}
        </p>
        <p className="text-white/70 text-sm sm:text-base">
          {config.location_addr}
        </p>
        <a
          href={config.website_url}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-block mt-6 text-primary-light underline text-sm hover:text-white transition-colors"
        >
          Campsite info ({new URL(config.website_url).hostname})
        </a>
      </div>
    </section>
  );
}

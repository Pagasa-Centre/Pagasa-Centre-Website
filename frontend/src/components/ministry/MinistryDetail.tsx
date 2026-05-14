import Image from "next/image";
import Link from "next/link";
import type { Ministry } from "@/types";
import MinistryInfoCard from "./MinistryInfoCard";

export default function MinistryDetail({ ministry }: { ministry: Ministry }) {
  const ctaHref = ministry.getInvolvedHref ?? "/schedule";
  const paragraphs = ministry.about ?? [ministry.description];

  return (
    <section className="relative pt-[88px] bg-white">
      {/* Hero image background — only covers the top portion */}
      <div className="absolute inset-x-0 top-0 h-[680px] z-0 overflow-hidden">
        {ministry.imageUrl && (
          <Image
            src={ministry.imageUrl}
            alt={ministry.name}
            fill
            priority
            className="object-cover object-center"
          />
        )}
        <div className="absolute inset-0 bg-black/55" />
      </div>

      <div className="relative z-10 max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="grid lg:grid-cols-[1fr_22rem] gap-10 lg:gap-12 items-start">
          {/* Left column: hero text + about */}
          <div>
            <div className="py-16 lg:py-24 text-white max-w-2xl">
              <p className="text-primary-light uppercase tracking-widest text-sm font-semibold mb-4">
                Get involved
              </p>
              <h1 className="text-4xl sm:text-5xl lg:text-6xl font-extrabold uppercase leading-tight mb-6 tracking-tight">
                {ministry.name}
              </h1>
              <p className="text-white/85 text-lg leading-relaxed mb-10">
                {ministry.description}
              </p>
              <Link
                href={ctaHref}
                className="inline-block px-10 py-4 bg-white text-neutral-900 font-bold uppercase tracking-widest text-sm hover:bg-primary hover:text-white transition-colors"
              >
                Get Involved
              </Link>
            </div>

            <div className="py-16 lg:py-24 max-w-2xl">
              <h2 className="text-3xl sm:text-4xl font-bold text-neutral-900 mb-8">
                About the ministry
              </h2>
              <div className="space-y-6 text-neutral-700 leading-relaxed">
                {paragraphs.map((p, i) => (
                  <p key={i}>{p}</p>
                ))}
              </div>
              <div className="mt-10">
                <Link
                  href={ctaHref}
                  className="inline-block px-10 py-4 bg-neutral-900 text-white font-bold uppercase tracking-widest text-sm hover:bg-primary transition-colors"
                >
                  Get Involved
                </Link>
              </div>
            </div>
          </div>

          {/* Right column: sticky info card spanning hero + about */}
          <div className="lg:sticky lg:top-28 self-start py-16 lg:py-24">
            <MinistryInfoCard ministry={ministry} />
          </div>
        </div>
      </div>
    </section>
  );
}

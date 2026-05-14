import Image from "next/image";
import Link from "next/link";
import type { Ministry } from "@/types";
import MinistryInfoCard from "./MinistryInfoCard";

// Hero/dark band height. Both the image background AND the hero-text column
// use the same height so the dark/white boundary lands exactly at the end
// of the hero text — not above or below it.
const HERO_HEIGHT = "min-h-[600px] sm:min-h-[640px] lg:min-h-[680px]";
const HERO_HEIGHT_BG = "h-[600px] sm:h-[640px] lg:h-[680px]";

export default function MinistryDetail({ ministry }: { ministry: Ministry }) {
  const ctaHref = ministry.getInvolvedHref ?? "/schedule";
  const sections = ministry.aboutSections ?? [{ body: [ministry.description] }];

  return (
    <section className="relative bg-white">
      {/* Full-width dark hero image, only behind the hero text */}
      <div
        className={`absolute inset-x-0 top-0 ${HERO_HEIGHT_BG} z-0 overflow-hidden`}
      >
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
            {/* Hero text — matches image bg height so the boundary lines up */}
            <div
              className={`${HERO_HEIGHT} pt-[120px] pb-12 flex items-center`}
            >
              <div className="text-white max-w-2xl">
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
            </div>

            {/* About — sits in the white area, well below the hero */}
            <div className="pt-16 lg:pt-24 pb-20 lg:pb-28 max-w-2xl">
              <h2 className="text-3xl sm:text-4xl font-bold text-neutral-900 mb-8">
                About the ministry
              </h2>
              <div className="space-y-8 text-neutral-700 leading-relaxed">
                {sections.map((section, i) => (
                  <div key={i} className="space-y-4">
                    {section.heading && (
                      <h3 className="text-xl sm:text-2xl font-bold text-neutral-900">
                        {section.heading}
                      </h3>
                    )}
                    {section.body.map((p, j) => (
                      <p key={j}>{p}</p>
                    ))}
                    {section.bullets && section.bullets.length > 0 && (
                      <ul className="list-disc pl-6 space-y-1">
                        {section.bullets.map((b, j) => (
                          <li key={j}>{b}</li>
                        ))}
                      </ul>
                    )}
                  </div>
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
          <div className="pt-[120px]">
            <div className="lg:sticky lg:top-28">
              <MinistryInfoCard ministry={ministry} />
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

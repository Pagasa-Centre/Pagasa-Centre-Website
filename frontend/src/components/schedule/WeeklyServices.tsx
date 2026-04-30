import Image from "next/image";
import Link from "next/link";
import { getMinistryById } from "@/lib/ministries";
import type { Ministry } from "@/types";

function CalendarIcon() {
  return (
    <svg
      className="w-4 h-4"
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"
      />
    </svg>
  );
}

function ClockIcon() {
  return (
    <svg
      className="w-4 h-4"
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
      />
    </svg>
  );
}

function FeatureCard({ ministry }: { ministry: Ministry }) {
  return (
    <article className="group flex flex-col bg-white rounded-2xl overflow-hidden border border-neutral-300 shadow-sm hover:shadow-lg hover:border-primary/40 transition-all">
      <div className="relative h-56 sm:h-64 bg-neutral-200 overflow-hidden">
        {ministry.imageUrl ? (
          <Image
            src={ministry.imageUrl}
            alt={ministry.name}
            fill
            sizes="(min-width: 1024px) 50vw, 100vw"
            className="object-cover group-hover:scale-105 transition-transform duration-500"
          />
        ) : (
          <div className="absolute inset-0 flex items-center justify-center">
            <span className="text-neutral-400 text-7xl font-bold">
              {ministry.name[0]}
            </span>
          </div>
        )}
        <div className="absolute top-4 left-4 inline-flex items-center gap-2 px-3 py-1.5 bg-primary text-white text-xs font-bold uppercase tracking-widest">
          <CalendarIcon />
          {ministry.day}
        </div>
      </div>

      <div className="flex-1 flex flex-col p-7">
        <div className="flex items-center gap-2 text-primary text-sm font-semibold mb-3">
          <ClockIcon />
          <span>{ministry.time}</span>
        </div>
        <h3 className="text-2xl font-extrabold text-neutral-900 mb-3 leading-tight group-hover:text-primary transition-colors">
          {ministry.name}
        </h3>
        <p className="text-neutral-600 leading-relaxed mb-6 flex-1">
          {ministry.description}
        </p>
        <Link
          href={`#${ministry.id}`}
          className="inline-flex items-center gap-2 text-primary font-semibold text-sm uppercase tracking-widest hover:gap-3 transition-all"
        >
          More information
          <svg
            className="w-4 h-4"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M9 5l7 7-7 7"
            />
          </svg>
        </Link>
      </div>
    </article>
  );
}

export default function WeeklyServices() {
  const sunday = getMinistryById("sunday-celebration");
  const wildsons = getMinistryById("wildsons");
  const featured: Ministry[] = [sunday, wildsons].filter(
    (m): m is Ministry => Boolean(m),
  );

  return (
    <section className="py-20 lg:py-24 bg-white">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-14">
          <div className="w-14 h-1 bg-neutral-800 mb-7 mx-auto" />
          <p className="text-primary uppercase tracking-widest text-sm font-semibold mb-3">
            Weekly Services
          </p>
          <h2 className="text-4xl sm:text-5xl font-extrabold text-neutral-900 leading-tight">
            See our schedule below
          </h2>
          <p className="mt-5 text-neutral-600 max-w-2xl mx-auto">
            Two weekly gatherings anchor our community. Whether you&apos;re new
            or have been with us for years, you&apos;re invited.
          </p>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
          {featured.map((m) => (
            <FeatureCard key={m.id} ministry={m} />
          ))}
        </div>
      </div>
    </section>
  );
}

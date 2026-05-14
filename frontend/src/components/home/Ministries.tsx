import Image from "next/image";
import Link from "next/link";
import { featuredMinistries } from "@/lib/ministries";

function CalendarIcon() {
  return (
    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
        d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
    </svg>
  );
}

function ClockIcon() {
  return (
    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
        d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  );
}

export default function Ministries() {
  return (
    <section className="py-20 bg-surface" id="ministries">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-14">
          <p className="text-primary uppercase tracking-widest text-sm font-semibold mb-2">
            Our Ministries
          </p>
          <h2 className="text-3xl sm:text-4xl font-bold text-neutral-800">
            Get involved in our movement
          </h2>
          <p className="mt-4 text-neutral-600 max-w-xl mx-auto">
            There is a place for everyone. Find a ministry where you can grow,
            serve, and belong.
          </p>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
          {featuredMinistries.map((m) => (
            <Link
              href={`/ministry/${m.id}`}
              key={m.id}
              className="bg-white rounded-xl shadow-sm border border-neutral-300 overflow-hidden hover:shadow-md hover:border-primary/30 transition-all group"
            >
              <div className="relative h-48 bg-neutral-200 overflow-hidden">
                {m.imageUrl ? (
                  <Image
                    src={m.imageUrl}
                    alt={m.name}
                    fill
                    sizes="(min-width: 1024px) 33vw, (min-width: 640px) 50vw, 100vw"
                    className="object-cover group-hover:scale-105 transition-transform duration-500"
                  />
                ) : (
                  <div className="absolute inset-0 flex items-center justify-center">
                    <span className="text-neutral-400 text-4xl font-bold">
                      {m.name[0]}
                    </span>
                  </div>
                )}
              </div>
              <div className="p-6">
                <h3 className="font-bold text-neutral-800 text-lg mb-2 group-hover:text-primary transition-colors">
                  {m.name}
                </h3>
                <div className="flex items-center gap-4 text-sm text-neutral-500 mb-3">
                  <span className="flex items-center gap-1">
                    <CalendarIcon />
                    {m.day}
                  </span>
                  <span className="flex items-center gap-1">
                    <ClockIcon />
                    {m.time}
                  </span>
                </div>
                <p className="text-neutral-600 text-sm leading-relaxed">
                  {m.description}
                </p>
              </div>
            </Link>
          ))}
        </div>

        <div className="mt-12 text-center">
          <Link
            href="/schedule#ministries"
            className="inline-flex items-center gap-2 text-primary font-semibold hover:underline"
          >
            Browse all ministries
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
          </Link>
        </div>
      </div>
    </section>
  );
}

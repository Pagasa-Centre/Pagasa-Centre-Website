import Image from "next/image";
import { locations } from "@/lib/locations";

function MapPinIcon() {
  return (
    <svg
      className="w-4 h-4 text-primary shrink-0 mt-0.5"
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z"
      />
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M15 11a3 3 0 11-6 0 3 3 0 016 0z"
      />
    </svg>
  );
}

function ClockIcon() {
  return (
    <svg
      className="w-4 h-4 text-primary shrink-0 mt-0.5"
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

export default function Locations() {
  return (
    <section className="py-20 lg:py-24 bg-white" id="locations">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-14">
          <p className="text-primary uppercase tracking-widest text-sm font-semibold mb-2">
            Find Us
          </p>
          <h2 className="text-3xl sm:text-4xl font-bold text-neutral-800">
            Our locations
          </h2>
          <p className="mt-4 text-neutral-600 max-w-xl mx-auto">
            With {locations.length} congregations across the UK, Ireland, and
            the Philippines, there&apos;s a Pag-Asa Centre family near you.
          </p>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
          {locations.map((loc) => (
            <article
              key={loc.id}
              className="rounded-xl border border-neutral-300 bg-surface hover:border-primary/40 hover:shadow-md transition-all flex flex-col overflow-hidden group"
            >
              <div className="relative aspect-[16/10] bg-neutral-200 overflow-hidden">
                {loc.imageUrl ? (
                  <Image
                    src={loc.imageUrl}
                    alt={loc.name}
                    fill
                    sizes="(min-width: 1024px) 33vw, (min-width: 640px) 50vw, 100vw"
                    className="object-cover group-hover:scale-105 transition-transform duration-500"
                  />
                ) : (
                  <div className="absolute inset-0 flex items-center justify-center bg-gradient-to-br from-primary/15 to-primary/5">
                    <MapPinIcon />
                  </div>
                )}
              </div>

              <div className="p-5 flex-1 flex flex-col">
                <h3 className="text-base font-bold text-neutral-900 mb-3 leading-snug group-hover:text-primary transition-colors">
                  {loc.name}
                </h3>

                <div className="space-y-2 text-sm flex-1">
                  <div className="flex gap-2">
                    <MapPinIcon />
                    <p className="text-neutral-600 leading-relaxed">
                      {loc.venue ? (
                        <>
                          <span className="font-medium text-neutral-700">
                            {loc.venue}
                          </span>
                          <span className="block text-neutral-500">
                            {loc.address}
                          </span>
                        </>
                      ) : (
                        loc.address
                      )}
                    </p>
                  </div>
                  <div className="flex gap-2">
                    <ClockIcon />
                    <p className="text-neutral-600">{loc.schedule}</p>
                  </div>
                </div>
              </div>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}

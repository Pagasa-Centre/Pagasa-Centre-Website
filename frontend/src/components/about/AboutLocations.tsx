import Image from "next/image";
import { locations } from "@/lib/locations";
import { googleMapsUrl } from "@/lib/maps";

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

function PhoneIcon() {
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
        d="M3 5a2 2 0 012-2h2.28a2 2 0 011.94 1.515l.7 2.808a2 2 0 01-.45 1.892l-1.27 1.27a16 16 0 006.586 6.586l1.27-1.27a2 2 0 011.892-.45l2.808.7A2 2 0 0121 16.72V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z"
      />
    </svg>
  );
}

function MailIcon() {
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
        d="M3 8l9 6 9-6M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
      />
    </svg>
  );
}

function ExternalLinkIcon() {
  return (
    <svg
      className="inline-block w-3 h-3 ml-1 align-baseline opacity-60"
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
      />
    </svg>
  );
}

export default function AboutLocations() {
  return (
    <section className="py-20 lg:py-28 bg-white" id="more-locations">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-14">
          <div className="w-14 h-1 bg-neutral-800 mb-7 mx-auto" />
          <p className="text-primary uppercase tracking-widest text-sm font-semibold mb-3">
            Find Us
          </p>
          <h2 className="text-4xl sm:text-5xl font-extrabold text-neutral-900 leading-tight">
            More locations
          </h2>
          <p className="mt-5 text-neutral-600 max-w-2xl mx-auto">
            With congregations across the UK, Ireland, and the Philippines,
            there&apos;s a Pag-Asa Centre family near you.
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {locations.map((loc) => (
            <article
              key={loc.id}
              className="rounded-xl border border-neutral-300 bg-surface hover:border-primary/40 hover:shadow-md transition-all flex flex-col overflow-hidden"
            >
              <div className="relative aspect-[16/10] bg-neutral-200 overflow-hidden">
                {loc.imageUrl ? (
                  <Image
                    src={loc.imageUrl}
                    alt={loc.name}
                    fill
                    sizes="(min-width: 1024px) 33vw, (min-width: 768px) 50vw, 100vw"
                    className="object-cover"
                  />
                ) : (
                  <div className="absolute inset-0 flex items-center justify-center bg-gradient-to-br from-primary/15 to-primary/5">
                    <MapPinIcon />
                  </div>
                )}
              </div>

              <div className="p-6 flex-1 flex flex-col">
                <h3 className="text-lg font-bold text-neutral-900 mb-4 leading-snug">
                  {loc.name}
                </h3>

                <div className="space-y-3 text-sm flex-1">
                  <div className="flex gap-2">
                    <MapPinIcon />
                    <a
                      href={googleMapsUrl(loc.venue, loc.address)}
                      target="_blank"
                      rel="noopener noreferrer"
                      aria-label={`Open ${loc.name} in Google Maps`}
                      className="text-neutral-700 hover:text-primary transition-colors group/addr"
                    >
                      {loc.venue && (
                        <p className="font-medium text-neutral-800 group-hover/addr:text-primary">
                          {loc.venue}
                        </p>
                      )}
                      <p className="text-neutral-600 group-hover/addr:underline">
                        {loc.address}
                        <ExternalLinkIcon />
                      </p>
                    </a>
                  </div>

                  <div className="flex gap-2">
                    <ClockIcon />
                    <p className="text-neutral-700">{loc.schedule}</p>
                  </div>

                  {loc.phone && (
                    <div className="flex gap-2">
                      <PhoneIcon />
                      <a
                        href={`tel:${loc.phone.replace(/\s+/g, "")}`}
                        className="text-neutral-700 hover:text-primary transition-colors"
                      >
                        {loc.phone}
                      </a>
                    </div>
                  )}

                  {loc.email && (
                    <div className="flex gap-2">
                      <MailIcon />
                      <a
                        href={`mailto:${loc.email}`}
                        className="text-neutral-700 hover:text-primary transition-colors break-all"
                      >
                        {loc.email}
                      </a>
                    </div>
                  )}
                </div>
              </div>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}

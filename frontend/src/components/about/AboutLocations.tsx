import type { LocationDetail } from "@/types";

const locations: LocationDetail[] = [
  {
    id: "dagenham",
    name: "Jo Richardson Community School, Dagenham",
    address: "Castle Green, Gale St, Dagenham RM9 4UN",
    schedule: "Sunday | 02:00PM - 05:00PM",
    phone: "+44 79 8494 8682",
    email: "pagasa_media@hotmail.co.uk",
  },
  {
    id: "bray",
    name: "Bray, Ireland",
    venue: "Bray Methodist Church",
    address: "Florence Road, Bray, Ireland, A98 YR84",
    schedule: "Sunday | 12:00PM - 2:00PM",
    phone: "+353 87 186 4957",
    email: "pagasacentreirl@gmail.com",
  },
  {
    id: "pampanga",
    name: "Pampanga, Philippines",
    venue: "2nd Floor of Juliez Manukan Building",
    address: "San Matias Highway, Santo Tomas, Pampanga, Philippines",
    schedule: "Sunday | 8:30AM - 10:30AM",
  },
  {
    id: "bedfordshire",
    name: "Bedfordshire, UK",
    venue: "Bunyan Road Christian Centre",
    address: "30 Bunyan Road, Kempston, Bedfordshire, MK42 8HL",
    schedule: "Saturday | 3:00PM - 6:00PM (Every other week)",
  },
  {
    id: "reading",
    name: "Reading, UK",
    venue: "Emmanuel Church Centre",
    address: "South Lake Crescent, Woodley, Reading, RG5 3QW",
    schedule: "Saturday | 2:30PM - 5:00PM",
  },
  {
    id: "harwich",
    name: "Harwich, UK",
    venue: "Mayflower Primary School Hall",
    address: "Main Road, Dovercourt Harwich, Essex, C012 4AJ",
    schedule: "Saturday | 2:00PM - 4:00PM (Every other week)",
  },
  {
    id: "banga",
    name: "Banga, South Cotabato, Philippines",
    venue: "Pag-Asa Centre Banga, Ruiz Compound",
    address: "Bgy Kusan, Barrio 8, Banga, South Cotabato, Philippines",
    schedule: "Sunday | 9:00AM - 11:00AM",
  },
  {
    id: "stratford",
    name: "Stratford Upon Avon, UK",
    venue: "Ken Kennett Centre",
    address: "100 Justins Avenue, CV37 0DA",
    schedule: "Sunday | 2:00PM - 6:00PM (Every other week)",
  },
  {
    id: "westmidlands",
    name: "West Midlands & Worcestershire, UK",
    venue: "The Old Library Centre",
    address: "65 Ombersley Street East, Droitwich Spa, WR9 8QS",
    schedule: "Saturday | 2:00PM - 6:00PM (Every other week)",
  },
  {
    id: "southend",
    name: "Southend-on-Sea, UK",
    venue: "The Cornerstone URC Church",
    address: "Bournemouth Park Road, Essex, SS2 5JL",
    schedule: "Saturday | 2:30PM onwards",
  },
];

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
              className="p-6 rounded-xl border border-neutral-300 bg-surface hover:border-primary/40 hover:shadow-md transition-all flex flex-col"
            >
              <h3 className="text-lg font-bold text-neutral-900 mb-4 leading-snug">
                {loc.name}
              </h3>

              <div className="space-y-3 text-sm flex-1">
                <div className="flex gap-2">
                  <MapPinIcon />
                  <div className="text-neutral-700">
                    {loc.venue && (
                      <p className="font-medium text-neutral-800">
                        {loc.venue}
                      </p>
                    )}
                    <p className="text-neutral-600">{loc.address}</p>
                  </div>
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
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}

import type { Location } from "@/types";

const locations: Location[] = [
  { id: "bray", city: "Bray", country: "Ireland", venue: "Bray Methodist Church", address: "Florence Road, Bray" },
  { id: "pampanga", city: "Pampanga", country: "Philippines", venue: "Juliez Manukan Building (2nd Floor)", address: "Santo Tomas, Pampanga" },
  { id: "bedfordshire", city: "Bedfordshire", country: "UK", address: "30 Bunyan Road, Kempston" },
  { id: "reading", city: "Reading", country: "UK", venue: "Emmanuel Church Centre", address: "South Lake Crescent, Woodley" },
  { id: "harwich", city: "Harwich", country: "UK", venue: "Mayflower Primary School Hall", address: "Main Road, Harwich" },
  { id: "stratford", city: "Stratford Upon Avon", country: "UK", venue: "Ken Kennett Centre", address: "100 Justins Avenue" },
  { id: "banga", city: "Banga, South Cotabato", country: "Philippines", venue: "Ruiz Compound", address: "Bgy Kusan, Banga" },
  { id: "westmidlands", city: "Droitwich Spa", country: "UK", venue: "The Old Library Centre", address: "West Midlands / Worcestershire" },
  { id: "southend", city: "Southend-on-Sea", country: "UK", venue: "The Cornerstone URC Church", address: "Bournemouth Park Road" },
];

const countryFlag: Record<string, string> = {
  UK: "🇬🇧",
  Ireland: "🇮🇪",
  Philippines: "🇵🇭",
};

function MapPinIcon() {
  return (
    <svg className="w-5 h-5 text-primary shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
        d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z" />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
        d="M15 11a3 3 0 11-6 0 3 3 0 016 0z" />
    </svg>
  );
}

export default function Locations() {
  return (
    <section className="py-20 bg-white" id="locations">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-14">
          <p className="text-primary uppercase tracking-widest text-sm font-semibold mb-2">
            Find Us
          </p>
          <h2 className="text-3xl sm:text-4xl font-bold text-neutral-800">
            Our locations
          </h2>
          <p className="mt-4 text-neutral-600 max-w-xl mx-auto">
            With 9 congregations across 3 countries, there's a Pag-Asa Centre
            family near you.
          </p>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
          {locations.map((loc) => (
            <div
              key={loc.id}
              className="flex gap-3 p-5 rounded-xl border border-neutral-300 hover:border-primary/40 hover:shadow-sm transition-all bg-surface group"
            >
              <MapPinIcon />
              <div>
                <div className="flex items-center gap-2 mb-1">
                  <h3 className="font-semibold text-neutral-800 text-sm group-hover:text-primary transition-colors">
                    {loc.city}
                  </h3>
                  <span className="text-base leading-none">{countryFlag[loc.country]}</span>
                </div>
                {loc.venue && (
                  <p className="text-neutral-500 text-xs font-medium">{loc.venue}</p>
                )}
                <p className="text-neutral-500 text-xs mt-0.5">{loc.address}</p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

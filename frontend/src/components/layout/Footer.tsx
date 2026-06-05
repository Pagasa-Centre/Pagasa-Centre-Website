import Link from "next/link";
import Image from "next/image";
import { googleMapsUrl } from "@/lib/maps";

const menuLinks = [
  { label: "Home", href: "/" },
  { label: "Schedule", href: "/schedule" },
  { label: "About", href: "/about" },
  { label: "Ministry", href: "/ministries" },
  { label: "Events", href: "/events" },
  { label: "Contact", href: "/contact" },
];

const socialLinks = [
  {
    label: "Facebook",
    href: "https://www.facebook.com/pagasacentre",
    icon: (
      <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
        <path d="M24 12.073c0-6.627-5.373-12-12-12s-12 5.373-12 12c0 5.99 4.388 10.954 10.125 11.854v-8.385H7.078v-3.47h3.047V9.43c0-3.007 1.792-4.669 4.533-4.669 1.312 0 2.686.235 2.686.235v2.953H15.83c-1.491 0-1.956.925-1.956 1.874v2.25h3.328l-.532 3.47h-2.796v8.385C19.612 23.027 24 18.062 24 12.073z" />
      </svg>
    ),
  },
  {
    label: "Instagram",
    href: "https://www.instagram.com/pagasacentre",
    icon: (
      <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
        <path d="M12 2.163c3.204 0 3.584.012 4.85.07 3.252.148 4.771 1.691 4.919 4.919.058 1.265.069 1.645.069 4.849 0 3.205-.012 3.584-.069 4.849-.149 3.225-1.664 4.771-4.919 4.919-1.266.058-1.644.07-4.85.07-3.204 0-3.584-.012-4.849-.07-3.26-.149-4.771-1.699-4.919-4.92-.058-1.265-.07-1.644-.07-4.849 0-3.204.013-3.583.07-4.849.149-3.227 1.664-4.771 4.919-4.919 1.266-.057 1.645-.069 4.849-.069zm0-2.163c-3.259 0-3.667.014-4.947.072-4.358.2-6.78 2.618-6.98 6.98-.059 1.281-.073 1.689-.073 4.948 0 3.259.014 3.668.072 4.948.2 4.358 2.618 6.78 6.98 6.98 1.281.058 1.689.072 4.948.072 3.259 0 3.668-.014 4.948-.072 4.354-.2 6.782-2.618 6.979-6.98.059-1.28.073-1.689.073-4.948 0-3.259-.014-3.667-.072-4.947-.196-4.354-2.617-6.78-6.979-6.98-1.281-.059-1.69-.073-4.949-.073zm0 5.838c-3.403 0-6.162 2.759-6.162 6.162s2.759 6.163 6.162 6.163 6.162-2.759 6.162-6.163c0-3.403-2.759-6.162-6.162-6.162zm0 10.162c-2.209 0-4-1.79-4-4 0-2.209 1.791-4 4-4s4 1.791 4 4c0 2.21-1.791 4-4 4zm6.406-11.845c-.796 0-1.441.645-1.441 1.44s.645 1.44 1.441 1.44c.795 0 1.439-.645 1.439-1.44s-.644-1.44-1.439-1.44z" />
      </svg>
    ),
  },
  {
    label: "YouTube",
    href: "https://www.youtube.com/@PagasaCentre",
    icon: (
      <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
        <path d="M23.498 6.186a3.016 3.016 0 00-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 00.502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 002.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 002.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z" />
      </svg>
    ),
  },
];

const locations = [
  { city: "Bray", country: "Ireland", address: "Bray Methodist Church, Florence Road, Bray, Ireland, A98 YR84", image: "/loc-bray.png" },
  { city: "Pampanga", country: "Philippines", address: "2nd Floor of Juliez Manukan Blds, San Matias Highway, Santo Tomas", image: "/loc-pampanga.jpg" },
  { city: "Bedfordshire", country: "UK", address: "30 Bunyan Road, Kempston, Bedford", image: "/loc-bedfordshire.png" },
  { city: "Reading", country: "UK", address: "Emmanuel Church Centre, South Lake Crescent, Woodley", image: "/loc-reading.jpg" },
  { city: "Harwich", country: "UK", address: "Mayflower Primary School Hall, Main Road, Harwich", image: "/loc-harwich.jpg" },
  { city: "Banga, South Cotabato", country: "Philippines", address: "Ruiz Compound, Bgy Kusan, Banga", image: "/loc-banga.jpg" },
  { city: "Droitwich Spa", country: "UK", address: "The Old Library Centre, West Midlands / Worcestershire", image: "/loc-droitwich.png" },
  { city: "Southend-on-Sea", country: "UK", address: "The Cornerstone URC Church, Bournemouth Park Road", image: "/loc-south.png" },
  { city: "Stratford Upon Avon", country: "UK", address: "Ken Kennett Centre, 100 Justins Avenue", image: null },
];

export default function Footer() {
  const year = new Date().getFullYear();

  return (
    <footer className="bg-ink text-white">
      {/* Divider */}
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="border-t border-white/10" />
      </div>

      {/* Main footer content */}
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-14">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-14">

          {/* Left — Menu */}
          <div>
            <h4 className="text-white font-bold text-base mb-6">Menu</h4>
            <div className="grid grid-cols-2 gap-x-10 gap-y-3 max-w-xs">
              {menuLinks.map((l) => (
                <Link
                  key={l.label}
                  href={l.href}
                  className="text-white/70 text-sm hover:text-white transition-colors"
                >
                  {l.label}
                </Link>
              ))}
            </div>

            {/* Social */}
            <div className="flex gap-3 mt-10">
              {socialLinks.map((s) => (
                <a
                  key={s.label}
                  href={s.href}
                  target="_blank"
                  rel="noopener noreferrer"
                  aria-label={s.label}
                  className="w-9 h-9 rounded-full bg-white/8 flex items-center justify-center hover:bg-primary transition-colors"
                >
                  {s.icon}
                </a>
              ))}
            </div>
          </div>

          {/* Right — Locations */}
          <div>
            <h4 className="text-white font-bold text-base mb-6">Locations</h4>
            <div className="flex flex-col gap-5 max-h-[520px] overflow-y-auto pr-1 scrollbar-thin">
              {locations.map((loc) => (
                <a
                  key={loc.city}
                  href={googleMapsUrl(`${loc.city}, ${loc.country}`, loc.address)}
                  target="_blank"
                  rel="noopener noreferrer"
                  aria-label={`Open ${loc.city}, ${loc.country} in Google Maps`}
                  className="flex gap-4 items-start group/loc"
                >
                  {/* Thumbnail */}
                  <div className="relative w-24 h-16 shrink-0 overflow-hidden bg-neutral-700">
                    {loc.image ? (
                      <Image
                        src={loc.image}
                        alt={loc.city}
                        fill
                        className="object-cover"
                      />
                    ) : (
                      <div className="w-full h-full bg-neutral-700 flex items-center justify-center">
                        <svg className="w-6 h-6 text-neutral-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5}
                            d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z" />
                        </svg>
                      </div>
                    )}
                  </div>
                  {/* Text */}
                  <div>
                    <p className="font-bold text-white text-sm leading-snug group-hover/loc:text-primary transition-colors">
                      {loc.city}, {loc.country}
                      <svg
                        className="inline-block w-3 h-3 ml-1 align-baseline opacity-50"
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
                    </p>
                    <p className="text-white/55 text-xs mt-1 leading-relaxed group-hover/loc:text-white/80 transition-colors">
                      {loc.address}
                    </p>
                  </div>
                </a>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* Bottom bar */}
      <div className="border-t border-white/8">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4 flex flex-col sm:flex-row items-center justify-between gap-2 text-white/30 text-xs">
          <p>© {year} Pag-Asa Centre. All rights reserved.</p>
          <p>
            Built by{" "}
            <a
              href="https://goldliontechnologies.com"
              target="_blank"
              rel="noopener noreferrer"
              className="hover:text-white transition-colors"
            >
              Goldlion Technologies
            </a>
          </p>
        </div>
      </div>
    </footer>
  );
}

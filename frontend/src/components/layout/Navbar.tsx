"use client";

import { useState } from "react";
import Link from "next/link";
import Image from "next/image";

const navLinks = [
  { label: "Home", href: "/" },
  { label: "About Us", href: "/about" },
  { label: "Schedule", href: "/schedule" },
  { label: "Attend Online", href: "https://www.youtube.com/@PagasaCentre", external: true },
  { label: "Events", href: "/events" },
  { label: "How Can I Help?", href: "/help" },
  { label: "Contact Us", href: "/contact" },
];

export default function Navbar() {
  const [menuOpen, setMenuOpen] = useState(false);

  return (
    <header className="fixed top-0 left-0 right-0 z-50 bg-transparent text-white">
      <nav className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex items-center justify-between h-[88px]">
        {/* Logo */}
        <Link href="/" className="flex items-center shrink-0">
          <Image
            src="/pagasa-logo.png"
            alt="Pag-Asa Centre"
            width={72}
            height={72}
            className="object-contain"
          />
        </Link>

        {/* Desktop links */}
        <ul className="hidden lg:flex items-center gap-7 text-xs font-semibold tracking-widest uppercase">
          {navLinks.map((link) => (
            <li key={link.label}>
              {link.external ? (
                <a
                  href={link.href}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-white/80 hover:text-white transition-colors"
                >
                  {link.label}
                </a>
              ) : (
                <Link
                  href={link.href}
                  className="text-white/80 hover:text-white transition-colors"
                >
                  {link.label}
                </Link>
              )}
            </li>
          ))}
        </ul>

        {/* CTA — white, square, uppercase */}
        <Link
          href="/schedule"
          className="hidden lg:inline-flex items-center px-6 py-3 bg-white text-neutral-900 font-bold text-xs uppercase tracking-widest hover:bg-primary hover:text-white transition-colors"
        >
          Get Involved
        </Link>

        {/* Hamburger */}
        <button
          onClick={() => setMenuOpen(!menuOpen)}
          className="lg:hidden p-2 rounded-sm hover:bg-ink-secondary transition-colors"
          aria-label="Toggle menu"
        >
          <span className="block w-5 h-0.5 bg-white mb-1.5" />
          <span className="block w-5 h-0.5 bg-white mb-1.5" />
          <span className="block w-5 h-0.5 bg-white" />
        </button>
      </nav>

      {/* Mobile menu */}
      {menuOpen && (
        <div className="lg:hidden bg-ink-secondary border-t border-white/10">
          <ul className="px-4 py-4 flex flex-col gap-4 text-xs font-semibold tracking-widest uppercase">
            {navLinks.map((link) => (
              <li key={link.label}>
                {link.external ? (
                  <a
                    href={link.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-white/70 hover:text-white transition-colors"
                    onClick={() => setMenuOpen(false)}
                  >
                    {link.label}
                  </a>
                ) : (
                  <Link
                    href={link.href}
                    className="text-white/70 hover:text-white transition-colors"
                    onClick={() => setMenuOpen(false)}
                  >
                    {link.label}
                  </Link>
                )}
              </li>
            ))}
            <li>
              <Link
                href="/schedule"
                className="inline-flex items-center px-6 py-3 bg-white text-neutral-900 font-bold text-xs uppercase tracking-widest hover:bg-neutral-100 transition-colors"
                onClick={() => setMenuOpen(false)}
              >
                Get Involved
              </Link>
            </li>
          </ul>
        </div>
      )}
    </header>
  );
}

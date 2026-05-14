"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import Image from "next/image";

const navLinks = [
  { label: "Home", href: "/" },
  { label: "About Us", href: "/about" },
  { label: "Schedule", href: "/schedule" },
  {
    label: "Attend Online",
    href: "https://www.youtube.com/@PagasaCentre",
    external: true,
  },
  { label: "Events", href: "/events" },
  { label: "How Can I Help?", href: "/help" },
  { label: "Contact Us", href: "/contact" },
];

export default function Navbar() {
  const [menuOpen, setMenuOpen] = useState(false);

  // Lock body scroll while the mobile menu is open.
  useEffect(() => {
    if (!menuOpen) return;
    const original = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = original;
    };
  }, [menuOpen]);

  const closeMenu = () => setMenuOpen(false);

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

        {/* Desktop CTA */}
        <Link
          href="/schedule"
          className="hidden lg:inline-flex items-center px-6 py-3 bg-white text-neutral-900 font-bold text-xs uppercase tracking-widest hover:bg-primary hover:text-white transition-colors"
        >
          Get Involved
        </Link>

        {/* Hamburger (only when menu closed) */}
        {!menuOpen && (
          <button
            onClick={() => setMenuOpen(true)}
            className="lg:hidden p-2 rounded-sm hover:bg-white/10 transition-colors"
            aria-label="Open menu"
            aria-expanded="false"
          >
            <span className="block w-6 h-0.5 bg-white mb-1.5" />
            <span className="block w-6 h-0.5 bg-white mb-1.5" />
            <span className="block w-6 h-0.5 bg-white" />
          </button>
        )}
      </nav>

      {/* Mobile full-screen menu */}
      {menuOpen && (
        <div className="lg:hidden fixed inset-0 z-50 bg-ink text-white flex flex-col">
          {/* Top bar inside menu: logo + close button */}
          <div className="flex items-center justify-between px-4 sm:px-6 h-[88px] shrink-0">
            <Link
              href="/"
              onClick={closeMenu}
              className="flex items-center shrink-0"
            >
              <Image
                src="/pagasa-logo.png"
                alt="Pag-Asa Centre"
                width={72}
                height={72}
                className="object-contain"
              />
            </Link>
            <button
              onClick={closeMenu}
              className="w-11 h-11 flex items-center justify-center bg-primary hover:bg-primary-dark text-white transition-colors"
              aria-label="Close menu"
              aria-expanded="true"
            >
              <svg
                className="w-5 h-5"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2.5}
                  d="M6 6l12 12M18 6L6 18"
                />
              </svg>
            </button>
          </div>

          {/* Links */}
          <nav className="flex-1 overflow-y-auto px-6 sm:px-8 py-6">
            <ul className="flex flex-col gap-7 text-2xl font-bold tracking-widest uppercase">
              {navLinks.map((link) => (
                <li key={link.label}>
                  {link.external ? (
                    <a
                      href={link.href}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-white hover:text-primary transition-colors"
                      onClick={closeMenu}
                    >
                      {link.label}
                    </a>
                  ) : (
                    <Link
                      href={link.href}
                      className="text-white hover:text-primary transition-colors"
                      onClick={closeMenu}
                    >
                      {link.label}
                    </Link>
                  )}
                </li>
              ))}
            </ul>
          </nav>

          {/* Bottom CTA */}
          <div className="px-6 sm:px-8 pb-10 pt-4 shrink-0">
            <Link
              href="/schedule"
              onClick={closeMenu}
              className="block w-full text-center px-6 py-4 bg-white text-neutral-900 font-bold text-sm uppercase tracking-widest hover:bg-primary hover:text-white transition-colors"
            >
              Get Involved
            </Link>
          </div>
        </div>
      )}
    </header>
  );
}

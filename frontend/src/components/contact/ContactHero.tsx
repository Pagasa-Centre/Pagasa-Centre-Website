import Image from "next/image";

function PhoneIcon() {
  return (
    <svg
      className="w-6 h-6 text-primary-light"
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
      className="w-6 h-6 text-primary-light"
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

function ClockIcon() {
  return (
    <svg
      className="w-6 h-6 text-primary-light"
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

export default function ContactHero() {
  return (
    <section className="relative min-h-[600px] flex items-center justify-center overflow-hidden pt-[88px]">
      <Image
        src="/hero-bg.jpg"
        alt="Pag-Asa Centre congregation in worship"
        fill
        priority
        className="object-cover object-center"
      />

      <div className="absolute inset-0 bg-black/55" />

      <div className="relative z-10 text-center text-white px-4 sm:px-6 max-w-4xl mx-auto py-20">
        <p className="text-primary-light uppercase tracking-widest text-sm font-semibold mb-4">
          Get involved
        </p>
        <h1 className="text-5xl sm:text-6xl lg:text-7xl font-extrabold leading-tight mb-6 tracking-tight">
          Get in touch
        </h1>
        <p className="text-white/85 text-lg sm:text-xl max-w-2xl mx-auto mb-12 leading-relaxed">
          Reach out to us for questions, prayer requests, or to learn more
          about Pag-Asa Centre. We&apos;re here to help.
        </p>

        <div className="grid grid-cols-1 sm:grid-cols-3 gap-6 max-w-3xl mx-auto">
          <a
            href="tel:+447984945682"
            className="flex flex-col items-center gap-2 p-6 bg-white/5 border border-white/15 hover:bg-white/10 hover:border-primary-light/50 transition-colors"
          >
            <PhoneIcon />
            <p className="text-white/60 uppercase tracking-widest text-xs font-semibold">
              Give us a call
            </p>
            <p className="text-white font-semibold">+44 7984 945682</p>
          </a>

          <a
            href="mailto:connect@pagasacentre.com"
            className="flex flex-col items-center gap-2 p-6 bg-white/5 border border-white/15 hover:bg-white/10 hover:border-primary-light/50 transition-colors"
          >
            <MailIcon />
            <p className="text-white/60 uppercase tracking-widest text-xs font-semibold">
              Send us an email
            </p>
            <p className="text-white font-semibold break-all">
              connect@pagasacentre.com
            </p>
          </a>

          <div className="flex flex-col items-center gap-2 p-6 bg-white/5 border border-white/15">
            <ClockIcon />
            <p className="text-white/60 uppercase tracking-widest text-xs font-semibold">
              Our schedule
            </p>
            <p className="text-white font-semibold">
              Sunday | 02:00PM &ndash; 05:00PM
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}

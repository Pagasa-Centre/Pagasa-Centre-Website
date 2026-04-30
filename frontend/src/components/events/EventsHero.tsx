import Image from "next/image";
import Link from "next/link";

export default function EventsHero() {
  return (
    <section className="relative min-h-[600px] flex items-center justify-center overflow-hidden pt-[88px]">
      <Image
        src="/hero-bg.jpg"
        alt="Pag-Asa Centre community gathered at an event"
        fill
        priority
        className="object-cover object-center"
      />

      <div className="absolute inset-0 bg-black/55" />

      <div className="relative z-10 text-center text-white px-4 sm:px-6 max-w-3xl mx-auto py-20">
        <p className="text-primary-light uppercase tracking-widest text-sm font-semibold mb-4">
          Get involved
        </p>
        <h1 className="text-5xl sm:text-6xl lg:text-7xl font-extrabold leading-tight mb-6 tracking-tight">
          Events
        </h1>
        <p className="text-white/85 text-lg sm:text-xl max-w-2xl mx-auto mb-10 leading-relaxed">
          Stay connected and informed about our vibrant community. Explore
          upcoming events that bring us together in joy and worship.
        </p>
        <div className="flex flex-col sm:flex-row gap-4 justify-center">
          <Link
            href="#events"
            className="px-10 py-4 bg-white text-neutral-900 font-bold uppercase tracking-widest text-sm hover:bg-primary hover:text-white transition-colors"
          >
            View Events
          </Link>
          <Link
            href="/about#more-locations"
            className="px-10 py-4 bg-neutral-900/60 border border-white/50 text-white font-bold uppercase tracking-widest text-sm hover:bg-primary hover:border-primary transition-colors"
          >
            Find Locations
          </Link>
        </div>
      </div>
    </section>
  );
}

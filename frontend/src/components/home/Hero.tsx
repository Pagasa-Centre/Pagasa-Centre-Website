import Image from "next/image";
import Link from "next/link";

export default function Hero() {
  return (
    <section className="relative min-h-screen flex items-center justify-center overflow-hidden">
      {/* Background image */}
      <Image
        src="/hero-bg.jpg"
        alt="Pag-Asa Centre congregation in worship"
        fill
        priority
        className="object-cover object-center"
      />

      {/* Dark overlay — matches the original site's dim */}
      <div className="absolute inset-0 bg-black/55" />

      {/* Content */}
      <div className="relative z-10 text-center text-white px-4 sm:px-6 max-w-3xl mx-auto">
        <h1 className="text-5xl sm:text-6xl lg:text-7xl font-extrabold leading-tight mb-6 tracking-tight">
          Welcome to
          <br />
          Pag-Asa Centre
        </h1>
        <p className="text-white/85 text-lg sm:text-xl max-w-xl mx-auto mb-10">
          We are thrilled to have you join us for worship and fellowship at our
          church.
        </p>
        <div className="flex flex-col sm:flex-row gap-4 justify-center">
          <Link
            href="/schedule"
            className="px-10 py-4 bg-white text-neutral-900 font-bold uppercase tracking-widest text-sm hover:bg-primary hover:text-white transition-colors"
          >
            Get Involved
          </Link>
          <Link
            href="/about"
            className="px-10 py-4 bg-neutral-900/60 border border-white/50 text-white font-bold uppercase tracking-widest text-sm hover:bg-primary hover:border-primary transition-colors"
          >
            About Us
          </Link>
        </div>
      </div>
    </section>
  );
}

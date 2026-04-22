import Image from "next/image";

export default function AboutHero() {
  return (
    <section className="relative h-[60vh] min-h-[420px] flex items-center justify-center overflow-hidden">
      <Image
        src="/hero-bg.jpg"
        alt="Pag-Asa Centre congregation"
        fill
        priority
        className="object-cover object-center"
      />
      <div className="absolute inset-0 bg-black/60" />
      <div className="relative z-10 text-center text-white px-4 sm:px-6 max-w-3xl mx-auto">
        <p className="text-xs font-semibold uppercase tracking-widest text-white/70 mb-4">
          About Us
        </p>
        <h1 className="text-4xl sm:text-5xl lg:text-6xl font-extrabold leading-tight tracking-tight">
          Join us on our journey
          <br />
          as we help one another
        </h1>
        <p className="mt-6 text-white/80 text-lg max-w-xl mx-auto">
          Making a difference in our local communities through faith, love, and service.
        </p>
      </div>
    </section>
  );
}

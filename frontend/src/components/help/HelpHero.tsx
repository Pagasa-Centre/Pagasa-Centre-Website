import Image from "next/image";

export default function HelpHero() {
  return (
    <section className="relative min-h-[600px] flex items-center justify-center overflow-hidden pt-[88px]">
      <Image
        src="/our-story.jpg"
        alt="Pag-Asa Centre community gathered together"
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
          How you can help
        </h1>
        <p className="text-white/85 text-lg sm:text-xl max-w-2xl mx-auto mb-10 leading-relaxed">
          Join us on our journey as we help one another and make a difference
          in our local communities. Come as you are, and let us help you
          discover your purpose and experience the transformative power of
          faith.
        </p>
        <div className="flex flex-col sm:flex-row gap-4 justify-center">
          <a
            href="https://www.crowdfunder.co.uk/p/pag-asa-centre-christian-church"
            target="_blank"
            rel="noopener noreferrer"
            className="px-10 py-4 bg-white text-neutral-900 font-bold uppercase tracking-widest text-sm hover:bg-primary hover:text-white transition-colors"
          >
            Support Church Building Project
          </a>
          <a
            href="mailto:pagasacentre07@gmail.com?subject=Fundraising%20Idea%20for%20Pag-Asa%20Centre"
            className="px-10 py-4 bg-neutral-900/60 border border-white/50 text-white font-bold uppercase tracking-widest text-sm hover:bg-primary hover:border-primary transition-colors"
          >
            Suggest Fundraising Ideas
          </a>
        </div>
      </div>
    </section>
  );
}

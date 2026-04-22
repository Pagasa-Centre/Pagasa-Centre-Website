import Image from "next/image";

export default function PrayerChain() {
  return (
    <section className="flex flex-col lg:flex-row min-h-[600px]" id="prayer">
      {/* Left — text content */}
      <div className="flex-1 flex items-start justify-end bg-white">
        <div className="w-full max-w-[680px] px-8 sm:px-14 lg:px-20 py-20 lg:py-28">
          {/* Decorative rule */}
          <div className="w-14 h-1 bg-neutral-800 mb-7" />

          <h2 className="text-4xl sm:text-5xl font-extrabold text-neutral-900 mb-8 leading-tight">
            Our 24Hr Prayer Chain
          </h2>

          <p className="text-neutral-700 text-base leading-relaxed">
            As part of our efforts as a Church to help out our community, we
            have started our very own 24-Hour Prayer Chain; with the intention
            of using continuous worldwide intercession to move mountains. Join
            us and together we can create a fortress of faith that can bring
            about abundant blessings, answered prayers and transformation in
            the lives of those in our community. To learn more about our prayer
            chain please ask your cell leader or the person that invited you!
          </p>
        </div>
      </div>

      {/* Right — Zoom screenshot, flush to edge, on dark background */}
      <div className="relative w-full lg:w-[45%] min-h-[400px] lg:min-h-0 bg-neutral-900">
        <Image
          src="/prayer-chain.jpg"
          alt="24-Hour Prayer Chain online meeting"
          fill
          className="object-cover object-center"
        />
      </div>
    </section>
  );
}

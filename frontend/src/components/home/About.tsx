import Link from "next/link";
import Image from "next/image";

export default function About() {
  return (
    <section className="flex flex-col lg:flex-row min-h-[600px]" id="about">
      {/* Left — text content */}
      <div className="flex-1 flex items-start justify-end bg-white">
        <div className="w-full max-w-[680px] px-8 sm:px-14 lg:px-20 py-20 lg:py-28">
          {/* Decorative rule */}
          <div className="w-14 h-1 bg-neutral-800 mb-7" />

          <h2 className="text-4xl sm:text-5xl font-extrabold text-neutral-900 mb-8 leading-tight">
            Our Story
          </h2>

          <div className="space-y-5 text-neutral-700 text-base leading-relaxed">
            <p>
              Pag-Asa Centre is a non-denominational church, established by God
              in the year 2007. As a church, we are passionate about God&apos;s
              presence, possess sincere integrity and a deep craving to reach
              the lost, walk in a spirit-filled faith, grounded in humility and
              brokenness. We are a registered charity institution by the Charity
              Commission of the United Kingdom.
            </p>
            <p>
              Pag-Asa Centre has a God-ordained mandate to present the Gospel of
              Christ Jesus to the lost, share the love of God, give hope to the
              hopeless, provide humanitarian aid to the hurting people, bring
              them to membership in His church, help them grow to Christ-like
              maturity, equip them for the ministry and send them in their life
              missions. (Matthew 28:28-20, John 10:10, Isaiah 1:17)
            </p>
          </div>

          <Link
            href="/about"
            className="inline-flex items-center mt-12 px-8 py-4 bg-neutral-900 text-white font-bold text-xs uppercase tracking-widest hover:bg-primary transition-colors"
          >
            Learn More
          </Link>
        </div>
      </div>

      {/* Right — image, flush to edge */}
      <div className="relative w-full lg:w-[45%] min-h-[400px] lg:min-h-0">
        <Image
          src="/our-story.jpg"
          alt="Pag-Asa Centre worship team"
          fill
          className="object-cover object-center"
        />
      </div>
    </section>
  );
}

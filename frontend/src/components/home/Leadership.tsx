import Image from "next/image";
import Link from "next/link";

export default function Leadership() {
  return (
    <section className="flex flex-col lg:flex-row min-h-[600px]" id="leadership">
      {/* Left — image, flush to edge */}
      <div className="relative w-full lg:w-[45%] min-h-[400px] lg:min-h-0">
        <Image
          src="/pastors.jpg"
          alt="Dr. Godofredo Ambat and Pastor Shirley Ambat"
          fill
          className="object-cover object-top"
        />
      </div>

      {/* Right — text content */}
      <div className="flex-1 flex items-start justify-start bg-white">
        <div className="w-full max-w-[680px] px-8 sm:px-14 lg:px-20 py-20 lg:py-28">
          {/* Decorative rule */}
          <div className="w-14 h-1 bg-neutral-800 mb-7" />

          <h2 className="text-4xl sm:text-5xl font-extrabold text-neutral-900 mb-8 leading-tight">
            Meet Dr. Ambat &amp; Pstr.
            <br />
            Shirley, our beloved
            <br />
            pastors
          </h2>

          <p className="text-neutral-700 text-base leading-relaxed mb-10">
            Dr. Godofredo Ambat &amp; Pstr. Shirley Ambat are our pastors and
            leaders in Pag-Asa Centre. Under their leadership, the church has
            grown both spiritually and exponentially through the working hand
            of the Holy Spirit. They are dedicated, compassionate and they love
            to serve the Lord, making a significant impact in the ministry.
          </p>

          <Link
            href="/about"
            className="inline-flex items-center px-8 py-4 bg-neutral-900 text-white font-bold text-xs uppercase tracking-widest hover:bg-neutral-700 transition-colors"
          >
            More About Us
          </Link>
        </div>
      </div>
    </section>
  );
}

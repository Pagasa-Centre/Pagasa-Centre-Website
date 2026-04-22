import Image from "next/image";

export default function AboutLeadership() {
  return (
    <section className="flex flex-col lg:flex-row min-h-[600px]" id="leadership">
      {/* Left — image */}
      <div className="relative w-full lg:w-[45%] min-h-[400px] lg:min-h-0">
        <Image
          src="/pastors.jpg"
          alt="Dr. Godofredo Ambat and Pastor Shirley Ambat"
          fill
          className="object-cover object-top"
        />
      </div>

      {/* Right — text */}
      <div className="flex-1 flex items-start justify-start bg-surface">
        <div className="w-full max-w-[680px] px-8 sm:px-14 lg:px-20 py-20 lg:py-28">
          <div className="w-14 h-1 bg-neutral-800 mb-7" />

          <h2 className="text-4xl sm:text-5xl font-extrabold text-neutral-900 mb-3 leading-tight">
            Our Leadership
          </h2>

          <div className="space-y-10 mt-8">
            {/* Bishop */}
            <div>
              <p className="text-xs font-semibold uppercase tracking-widest text-primary mb-1">
                Bishop &amp; Senior Pastor
              </p>
              <h3 className="text-2xl font-extrabold text-neutral-900 mb-3">
                Dr. Godofredo Ambat
              </h3>
              <p className="text-neutral-700 text-base leading-relaxed">
                Dr. Godofredo Ambat is the founding bishop and senior pastor of
                Pag-Asa Centre. Under his vision and servant leadership, the
                church has grown from a handful of believers to a network of
                congregations spanning the UK, Ireland, and the Philippines.
                His heart for the lost and dedication to discipleship have
                shaped the spiritual DNA of the entire ministry.
              </p>
            </div>

            {/* Pastora */}
            <div>
              <p className="text-xs font-semibold uppercase tracking-widest text-primary mb-1">
                Pastora
              </p>
              <h3 className="text-2xl font-extrabold text-neutral-900 mb-3">
                Pstr. Shay Ambat
              </h3>
              <p className="text-neutral-700 text-base leading-relaxed">
                Pstr. Shay Ambat co-leads the ministry alongside Bishop Ambat,
                bringing a compassionate and nurturing presence to every
                congregation. She is dedicated to building community, supporting
                families, and raising up the next generation of leaders within
                the church.
              </p>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

import Image from "next/image";

interface PastorCardProps {
  name: string;
  title: string;
  bio: string;
  imageUrl: string;
}

function PastorCard({ name, title, bio, imageUrl }: PastorCardProps) {
  return (
    <article className="bg-white border border-neutral-300 overflow-hidden flex flex-col">
      <div className="relative w-full aspect-[4/5]">
        <Image
          src={imageUrl}
          alt={name}
          fill
          className="object-cover object-center"
          sizes="(min-width: 768px) 50vw, 100vw"
        />
      </div>
      <div className="p-8 flex-1 flex flex-col">
        <h3 className="text-2xl font-extrabold text-neutral-900 mb-2">
          {name}
        </h3>
        <p className="text-primary uppercase tracking-widest text-xs font-semibold mb-4">
          {title}
        </p>
        <p className="text-neutral-700 text-sm leading-relaxed">{bio}</p>
      </div>
    </article>
  );
}

export default function Pastors() {
  return (
    <section className="py-20 lg:py-28 bg-surface" id="pastors">
      <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-14">
          <div className="w-14 h-1 bg-neutral-800 mb-7 mx-auto" />
          <h2 className="text-4xl sm:text-5xl font-extrabold text-neutral-900 mb-4 leading-tight">
            Meet Dr. Ambat &amp; Pstr. Shay,
            <br />
            our beloved pastors
          </h2>
          <p className="text-neutral-600 max-w-2xl mx-auto">
            Guiding our congregation with faith, wisdom, and love.
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-8 max-w-4xl mx-auto">
          <PastorCard
            name="Dr. Godofredo Ambat"
            title="Bishop — Senior Pastor"
            bio="Guiding our spiritual journey with his wisdom and compassion. Under his leadership, the church has grown both spiritually and exponentially through the working hand of the Holy Spirit."
            imageUrl="/pastor-godofredo.jpg"
          />
          <PastorCard
            name="Pstr. Shay Ambat"
            title="Pastora"
            bio="Leading with grace and nurturing our community's faith. She is dedicated, compassionate and loves to serve the Lord, making a significant impact in the ministry alongside Dr. Ambat."
            imageUrl="/pastor-shay.jpg"
          />
        </div>
      </div>
    </section>
  );
}

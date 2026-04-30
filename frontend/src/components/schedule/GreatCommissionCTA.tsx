import Link from "next/link";

export default function GreatCommissionCTA() {
  return (
    <section className="bg-ink text-white">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-20 lg:py-28">
        <div className="text-center max-w-3xl mx-auto">
          <p className="text-primary-light uppercase tracking-widest text-sm font-semibold mb-4">
            Get involved
          </p>
          <h2 className="text-4xl sm:text-5xl lg:text-6xl font-extrabold leading-tight mb-8 tracking-tight">
            Join to the great commission of Christ
          </h2>
          <p className="text-white/75 text-lg max-w-2xl mx-auto mb-10 leading-relaxed">
            Every member is a minister. Step into your calling, serve alongside
            your church family, and be part of what God is doing in our
            communities.
          </p>
          <div className="flex flex-col sm:flex-row gap-4 justify-center">
            <Link
              href="/help"
              className="px-10 py-4 bg-primary text-white font-bold uppercase tracking-widest text-sm hover:bg-primary-dark transition-colors"
            >
              Get Involved
            </Link>
            <Link
              href="/about#more-locations"
              className="px-10 py-4 bg-transparent border border-white/50 text-white font-bold uppercase tracking-widest text-sm hover:bg-white hover:text-neutral-900 hover:border-white transition-colors"
            >
              Locations
            </Link>
          </div>
        </div>
      </div>
    </section>
  );
}

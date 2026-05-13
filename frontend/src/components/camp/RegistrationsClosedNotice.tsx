import Link from "next/link";

export default function RegistrationsClosedNotice() {
  return (
    <section className="bg-surface py-20 lg:py-28">
      <div className="max-w-2xl mx-auto px-4 sm:px-6 lg:px-8 text-center">
        <div className="w-14 h-1 bg-neutral-800 mb-7 mx-auto" />
        <p className="text-primary uppercase tracking-widest text-sm font-semibold mb-3">
          Registrations Closed
        </p>
        <h2 className="text-3xl sm:text-4xl font-bold text-neutral-900 leading-tight mb-5">
          Camp registration is not currently open
        </h2>
        <p className="text-neutral-600 mb-8">
          Please check back soon or get in touch with your cell or network
          leader for the latest details.
        </p>
        <Link
          href="/contact"
          className="inline-flex items-center px-8 py-3 bg-primary text-white font-bold uppercase tracking-widest text-sm hover:bg-primary-dark transition-colors"
        >
          Contact us
        </Link>
      </div>
    </section>
  );
}

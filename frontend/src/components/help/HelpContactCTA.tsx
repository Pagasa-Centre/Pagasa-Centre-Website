export default function HelpContactCTA() {
  return (
    <section className="bg-ink text-white" id="contact">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-20 lg:py-28">
        <div className="text-center max-w-3xl mx-auto">
          <p className="text-primary-light uppercase tracking-widest text-sm font-semibold mb-4">
            Get in touch
          </p>
          <h2 className="text-4xl sm:text-5xl lg:text-6xl font-extrabold leading-tight mb-8 tracking-tight">
            Have a question?
          </h2>
          <p className="text-white/75 text-lg max-w-2xl mx-auto mb-10 leading-relaxed">
            If you have any queries, feel free to email Sister Lotta Jover.
            Thank you in advance and may God bless you abundantly.
          </p>
          <div className="flex justify-center">
            <a
              href="mailto:pagasacentre07@gmail.com?subject=Pag-Asa%20Centre%20Enquiry"
              className="px-10 py-4 bg-primary text-white font-bold uppercase tracking-widest text-sm hover:bg-primary-dark transition-colors"
            >
              Email Sister Lotta
            </a>
          </div>
          <p className="mt-8 text-primary-light italic text-sm tracking-wide">
            3 John 1:2
          </p>
        </div>
      </div>
    </section>
  );
}

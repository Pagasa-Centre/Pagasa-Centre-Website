"use client";

export default function ContactForm() {
  function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const name = String(fd.get("name") || "");
    const email = String(fd.get("email") || "");
    const phone = String(fd.get("phone") || "");
    const subject = String(fd.get("subject") || "Pag-Asa Centre Enquiry");
    const message = String(fd.get("message") || "");

    const encodedSubject = encodeURIComponent(subject);
    const encodedBody = encodeURIComponent(
      `Name: ${name}\nEmail: ${email}\nPhone: ${phone}\n\n${message}`,
    );

    window.location.href = `mailto:connect@pagasacentre.com?subject=${encodedSubject}&body=${encodedBody}`;
  }

  return (
    <section className="bg-surface py-20 lg:py-28" id="contact-form">
      <div className="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-12">
          <div className="w-14 h-1 bg-neutral-800 mb-7 mx-auto" />
          <p className="text-primary uppercase tracking-widest text-sm font-semibold mb-3">
            Send a message
          </p>
          <h2 className="text-4xl sm:text-5xl font-extrabold text-neutral-900 leading-tight">
            We&apos;d love to hear from you
          </h2>
          <p className="mt-5 text-neutral-600 max-w-xl mx-auto">
            Fill in the form below and your message will open in your email
            client, ready to send to our team.
          </p>
        </div>

        <form
          onSubmit={handleSubmit}
          className="bg-white border border-neutral-300 p-6 sm:p-10 rounded-xl shadow-sm flex flex-col gap-5"
        >
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
            <div className="flex flex-col gap-2">
              <label
                htmlFor="contact-name"
                className="text-xs font-bold uppercase tracking-widest text-neutral-700"
              >
                Name
              </label>
              <input
                id="contact-name"
                type="text"
                name="name"
                required
                placeholder="Your name"
                className="px-4 py-3 bg-white border border-neutral-300 text-neutral-900 placeholder-neutral-500 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary"
              />
            </div>
            <div className="flex flex-col gap-2">
              <label
                htmlFor="contact-email"
                className="text-xs font-bold uppercase tracking-widest text-neutral-700"
              >
                Email
              </label>
              <input
                id="contact-email"
                type="email"
                name="email"
                required
                placeholder="you@example.com"
                className="px-4 py-3 bg-white border border-neutral-300 text-neutral-900 placeholder-neutral-500 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary"
              />
            </div>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
            <div className="flex flex-col gap-2">
              <label
                htmlFor="contact-phone"
                className="text-xs font-bold uppercase tracking-widest text-neutral-700"
              >
                Phone number
              </label>
              <input
                id="contact-phone"
                type="tel"
                name="phone"
                placeholder="Optional"
                className="px-4 py-3 bg-white border border-neutral-300 text-neutral-900 placeholder-neutral-500 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary"
              />
            </div>
            <div className="flex flex-col gap-2">
              <label
                htmlFor="contact-subject"
                className="text-xs font-bold uppercase tracking-widest text-neutral-700"
              >
                Subject
              </label>
              <input
                id="contact-subject"
                type="text"
                name="subject"
                required
                placeholder="What's this about?"
                className="px-4 py-3 bg-white border border-neutral-300 text-neutral-900 placeholder-neutral-500 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary"
              />
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <label
              htmlFor="contact-message"
              className="text-xs font-bold uppercase tracking-widest text-neutral-700"
            >
              Message
            </label>
            <textarea
              id="contact-message"
              name="message"
              required
              rows={6}
              placeholder="How can we help you?"
              className="px-4 py-3 bg-white border border-neutral-300 text-neutral-900 placeholder-neutral-500 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary resize-y"
            />
          </div>

          <div className="flex justify-center pt-2">
            <button
              type="submit"
              className="px-10 py-4 bg-primary text-white font-bold uppercase tracking-widest text-sm hover:bg-primary-dark transition-colors"
            >
              Send Message
            </button>
          </div>
        </form>
      </div>
    </section>
  );
}

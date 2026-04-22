import type { Testimonial } from "@/types";

const testimonials: Testimonial[] = [
  {
    id: "1",
    name: "Lionel Wilson",
    location: "London",
    quote:
      "Pag-Asa Centre is a warm, lovely, and supportive community. I have found a true church family here.",
  },
  {
    id: "2",
    name: "Gee Cheung",
    location: "London",
    quote:
      "Pag-Asa Centre feels like home. From the very first service, I knew this was where I belonged.",
  },
  {
    id: "3",
    name: "Dhrubo Barua",
    location: "London",
    quote:
      "This church helped me find a deeper connection with Christ and a real sense of belonging in the community.",
  },
];

function QuoteIcon() {
  return (
    <svg className="w-8 h-8 text-primary/25" fill="currentColor" viewBox="0 0 24 24">
      <path d="M14.017 21v-7.391c0-5.704 3.731-9.57 8.983-10.609l.995 2.151c-2.432.917-3.995 3.638-3.995 5.849h4v10h-9.983zm-14.017 0v-7.391c0-5.704 3.748-9.57 9-10.609l.996 2.151c-2.433.917-3.996 3.638-3.996 5.849h3.983v10h-9.983z" />
    </svg>
  );
}

export default function Testimonials() {
  return (
    <section className="py-20 bg-surface" id="testimonials">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-14">
          <p className="text-primary uppercase tracking-widest text-sm font-semibold mb-2">
            Testimonials
          </p>
          <h2 className="text-3xl sm:text-4xl font-bold text-neutral-800">
            What our community says
          </h2>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          {testimonials.map((t) => (
            <div
              key={t.id}
              className="bg-white rounded-2xl p-8 shadow-sm border border-neutral-300 hover:shadow-md hover:border-primary/30 transition-all flex flex-col"
            >
              <QuoteIcon />
              <p className="text-neutral-600 leading-relaxed mt-4 flex-1 italic">
                &ldquo;{t.quote}&rdquo;
              </p>
              <div className="mt-6 flex items-center gap-3">
                <div className="w-10 h-10 rounded-full bg-primary/10 flex items-center justify-center shrink-0">
                  <span className="text-primary font-bold text-sm">{t.name[0]}</span>
                </div>
                <div>
                  <p className="font-semibold text-neutral-800 text-sm">{t.name}</p>
                  <p className="text-neutral-500 text-xs">{t.location}</p>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

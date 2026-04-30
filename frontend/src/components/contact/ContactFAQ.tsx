const faqs: { question: string; answer: string }[] = [
  {
    question: "Is there a dress code for attending services?",
    answer: "There is no dress code. Come as you are!",
  },
  {
    question: "How can I get involved in ministry?",
    answer:
      "We welcome servant leaders! Visit our \u201CGet Involved\u201D page or speak with the person who invited you.",
  },
  {
    question: "Do you have programs for children, youth, or young adults?",
    answer:
      "Yes \u2014 see our Children\u2019s Ministry. Ages 12\u201316 join the Children\u2019s Ministry; 16+ join Wildsons; young professionals connect through Crossover.",
  },
  {
    question: "How can I donate to the church?",
    answer:
      "You can give online through our website. God bless your generosity!",
  },
  {
    question: "Are your services accessible online?",
    answer:
      "Yes, we livestream our services. Use the \u201CAttend Online\u201D link in the navigation.",
  },
  {
    question: "How do I join a Cell group or Bible study?",
    answer:
      "Reach out to one of our network leaders \u2014 they will welcome you and guide you on the journey.",
  },
];

function ChevronIcon() {
  return (
    <svg
      className="w-5 h-5 text-primary shrink-0 transition-transform group-open:rotate-180"
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M19 9l-7 7-7-7"
      />
    </svg>
  );
}

export default function ContactFAQ() {
  return (
    <section className="py-20 lg:py-28 bg-white" id="faq">
      <div className="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-12">
          <div className="w-14 h-1 bg-neutral-800 mb-7 mx-auto" />
          <p className="text-primary uppercase tracking-widest text-sm font-semibold mb-3">
            Questions
          </p>
          <h2 className="text-4xl sm:text-5xl font-extrabold text-neutral-900 leading-tight">
            Frequently asked questions
          </h2>
          <p className="mt-5 text-neutral-600 max-w-2xl mx-auto">
            Can&apos;t find what you&apos;re looking for? Send us a message
            using the form above and we&apos;ll get back to you.
          </p>
        </div>

        <div className="border-t border-neutral-300">
          {faqs.map((f) => (
            <details
              key={f.question}
              className="group border-b border-neutral-300 py-5"
            >
              <summary className="flex justify-between items-center gap-4 cursor-pointer list-none text-lg font-bold text-neutral-900 [&::-webkit-details-marker]:hidden hover:text-primary transition-colors">
                <span>{f.question}</span>
                <ChevronIcon />
              </summary>
              <p className="mt-3 text-neutral-700 leading-relaxed">
                {f.answer}
              </p>
            </details>
          ))}
        </div>
      </div>
    </section>
  );
}

const beliefs = [
  {
    number: "01",
    title: "One God",
    body: "We believe in one God — Father, Son, and Holy Spirit — eternally existing as a holy Trinity.",
  },
  {
    number: "02",
    title: "The Triune Nature of God",
    body: "We believe in the full deity and distinct personhood of the Father, the Son, and the Holy Spirit.",
  },
  {
    number: "03",
    title: "Scripture",
    body: "We believe the Holy Scriptures of the Old and New Testaments are the inspired, infallible Word of God.",
  },
  {
    number: "04",
    title: "Human Sinfulness",
    body: "We believe that all have sinned and fallen short of the glory of God, yet redemption is found in Christ alone.",
  },
  {
    number: "05",
    title: "Salvation",
    body: "We believe that salvation requires spiritual rebirth through the Holy Spirit, producing genuine transformation and good works.",
  },
  {
    number: "06",
    title: "Christ's Return",
    body: "We believe in the personal and imminent return of Jesus Christ in two phases: first for His Church, then with His Church.",
  },
  {
    number: "07",
    title: "Eternal Destinations",
    body: "We believe in the resurrection of the saved and the lost — heaven and hell are real and consequential realities.",
  },
  {
    number: "08",
    title: "Baptism",
    body: "We believe in water baptism and baptism in the Holy Spirit as the right and privilege of every born-again believer.",
  },
  {
    number: "09",
    title: "The Family",
    body: "We believe in the sanctity of the family as a divine institution reflecting God's eternal covenant love.",
  },
  {
    number: "10",
    title: "Marriage",
    body: "We believe that marriage is a sacred, lifelong union between one man and one woman as ordained by God.",
  },
  {
    number: "11",
    title: "The Great Commission",
    body: "We believe in the mandate to go and make disciples of all nations, baptising them and teaching them to obey all Christ has commanded.",
  },
  {
    number: "12",
    title: "Christian Unity",
    body: "We believe all born-again believers are brethren in Christ Jesus, united in one body regardless of background or denomination.",
  },
];

export default function StatementOfFaith() {
  return (
    <section className="bg-surface" id="statement-of-faith">
      <div className="max-w-7xl mx-auto px-8 sm:px-14 lg:px-20 py-20 lg:py-28">
        <div className="w-14 h-1 bg-neutral-800 mb-7" />

        <div className="mb-14">
          <h2 className="text-4xl sm:text-5xl font-extrabold text-neutral-900 leading-tight mb-4">
            Statement of Faith
          </h2>
          <p className="text-neutral-600 text-base max-w-xl">
            Twelve core convictions that shape everything we believe, teach, and live as a community.
          </p>
        </div>

        <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-8">
          {beliefs.map(({ number, title, body }) => (
            <div
              key={number}
              className="bg-white p-8 border border-neutral-300/60"
            >
              <p className="text-4xl font-extrabold text-neutral-300 mb-4 leading-none">
                {number}
              </p>
              <h3 className="text-base font-bold text-neutral-900 uppercase tracking-wider mb-3">
                {title}
              </h3>
              <p className="text-neutral-600 text-sm leading-relaxed">{body}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

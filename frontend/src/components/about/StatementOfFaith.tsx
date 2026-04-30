import type { Belief } from "@/types";

const beliefs: Belief[] = [
  {
    number: 1,
    title: "We Believe in God! \"Elohim\"",
    description:
      "We believe that there is only one GOD eternally existent in three persons – Father, Son and Holy Spirit. He is the Creator of Heaven and Earth.",
    scripture: "Genesis 1:1, 26",
  },
  {
    number: 2,
    title: "We Believe in the Triune God",
    description:
      "God the Father, God the Son and God the Holy Spirit.",
    scripture: "Genesis 1:1; Matthew 3:16-17, 28-19; John 14-16",
  },
  {
    number: 3,
    title: "We Believe in the Infallibility & Immutability of the Scripture",
    description:
      "We believe that the Holy Scriptures are the inspired Word of God. They are the supreme and final authority for faith and practice.",
    scripture:
      "Numbers 23:19; Malachi 3:6; Psalms 19:7; Proverbs 19:21; 2 Timothy 3:16; 2 Peter 1:19-21",
  },
  {
    number: 4,
    title: "We Believe in the Sinfulness of Man",
    description:
      "We believe that although all people are sinners and have fallen short of God's glory, there is hope. Through faith in Jesus Christ, we can be forgiven of our sins and reconciled to God.",
    scripture: "Romans 3:9-18, 23, 5:12; 1 John 1:8-10; Isaiah 64:6",
  },
  {
    number: 5,
    title:
      "We Believe in the Need for the New Birth through the Holy Spirit in Christ",
    description:
      "We believe that good works are not the means of salvation but are the expected product in the life of a true believer in Christ. It is every believer's responsibility to pursue a life of good works through the power of the indwelling Holy Spirit.",
    scripture: "John 3:1-8; 2 Corinthians 5:17",
  },
  {
    number: 6,
    title: "We Believe in the Second Coming of Christ",
    description:
      "We believe in the literal, personal, visible return of our Lord Jesus Christ in two phases — to gather His Church and ultimately to reign.",
    scripture:
      "Matthew 24:14-31; Luke 21:34-36; 1 Thessalonians 4:16-17; Acts 1:9-11; Revelation 19:11-21",
  },
  {
    number: 7,
    title: "We Believe in Heaven & Hell",
    description:
      "We believe that water baptism by immersion is an act of obedience to Christ's command. It is a public confession of our personal faith in Jesus Christ.",
    scripture: "Luke 16:19-23; John 5:25-29; Revelation 20:13-15, 21:8",
  },
  {
    number: 8,
    title:
      "We Believe in the Baptism of Water and the Baptism in the Holy Spirit",
    description:
      "Public declaration of the new identity in Christ & the baptism of the Holy Spirit for the anointing of power to be witness for Christ Jesus.",
    scripture: "Matthew 3:13-15; Romans 6:3-8; Acts 1:8, 2:1-47; Matthew 3:11",
  },
  {
    number: 9,
    title: "We Believe in the Sanctity of the Family",
    description:
      "We believe that the family is a divine institution ordained by God. It is a cornerstone of society and should be a reflection of God's own eternal love.",
    scripture:
      "Exodus 20:12; Colossians 3:20-21; 1 Timothy 3:4; Ephesians 6:1-4; Acts 10:2; Joshua 24:15",
  },
  {
    number: 10,
    title: "We Believe in the Sanctity of Marriage",
    description:
      "We believe that marriage is a sacred institution ordained by God as a lifelong union between one man and one woman. It is a reflection of the eternal love between God and His people.",
    scripture: "Genesis 2:21-25; Isaiah 62:5",
  },
  {
    number: 11,
    title: "We Believe in the Great Commission",
    description:
      "We understand the Great Commission as a mandate from God to share the Gospel with all people, both near and far, and to make disciples who will in turn make disciples.",
    scripture: "Matthew 28:18-20; Mark 16:14-18",
  },
  {
    number: 12,
    title:
      "We Believe That All True Born Again Christians Are Our Brethren",
    description:
      "We are united by our common faith in Jesus Christ and our shared experience of salvation. We are called to work together to advance the kingdom of God.",
    scripture:
      "John 13:34-35, 15:9-13, 17; 1 John 3:18; Galatians 5:13; 1 Peter 4:8; Romans 12:10",
  },
];

export default function StatementOfFaith() {
  return (
    <section className="py-20 lg:py-28 bg-surface" id="statement-of-faith">
      <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-14">
          <div className="w-14 h-1 bg-neutral-800 mb-7 mx-auto" />
          <p className="text-primary uppercase tracking-widest text-sm font-semibold mb-3">
            What we believe
          </p>
          <h2 className="text-4xl sm:text-5xl font-extrabold text-neutral-900 leading-tight">
            Statement of Faith
          </h2>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6 lg:gap-8">
          {beliefs.map((belief) => (
            <article
              key={belief.number}
              className="bg-white p-6 sm:p-8 border-l-4 border-primary shadow-sm"
            >
              <span className="text-primary font-bold text-sm">
                {belief.number}.
              </span>
              <h3 className="text-base sm:text-lg font-bold uppercase text-neutral-900 mt-2 mb-3 tracking-wide leading-snug">
                {belief.title}
              </h3>
              <p className="text-neutral-700 text-sm leading-relaxed mb-4">
                {belief.description}
              </p>
              <p className="text-neutral-500 text-xs italic">
                {belief.scripture}
              </p>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}

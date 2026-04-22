export default function Sermons() {
  return (
    <section className="py-20 bg-white" id="sermons">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-14">
          <p className="text-primary uppercase tracking-widest text-sm font-semibold mb-2">
            Sermons & Worship
          </p>
          <h2 className="text-3xl sm:text-4xl font-bold text-neutral-800">
            Watch our latest messages
          </h2>
          <p className="mt-4 text-neutral-600 max-w-xl mx-auto">
            Can't make it in person? Watch our live-streamed services and
            sermon recordings online.
          </p>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8 items-center">
          {/* Video player placeholder */}
          <div className="relative aspect-video bg-ink rounded-2xl overflow-hidden shadow-xl group cursor-pointer">
            <div className="absolute inset-0 flex items-center justify-center">
              <div className="absolute inset-0 bg-[radial-gradient(ellipse_80%_60%_at_50%_50%,rgba(62,164,99,0.15),transparent)]" />
              <a
                href="https://www.youtube.com/@PagasaCentre"
                target="_blank"
                rel="noopener noreferrer"
                className="relative w-16 h-16 rounded-full bg-primary flex items-center justify-center shadow-lg group-hover:scale-110 group-hover:bg-primary-dark transition-all"
              >
                <svg className="w-6 h-6 text-white ml-1" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M8 5v14l11-7z" />
                </svg>
              </a>
            </div>
            <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/60 p-4">
              <p className="text-white text-sm font-medium">Latest sermon — Pag-Asa Centre</p>
              <p className="text-white/50 text-xs">Watch on YouTube</p>
            </div>
          </div>

          {/* Info */}
          <div className="space-y-6">
            {[
              {
                icon: (
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                    d="M15 10l4.553-2.277A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M3 8a2 2 0 012-2h8a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V8z" />
                ),
                title: "Live Streaming",
                text: "Every Sunday at 2:00 PM — watch live from anywhere in the world on our YouTube channel.",
              },
              {
                icon: (
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                    d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                ),
                title: "Sermon Archive",
                text: "Browse our full library of past messages and series at your own pace.",
              },
              {
                icon: (
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                    d="M3.055 11H5a2 2 0 012 2v1a2 2 0 002 2 2 2 0 012 2v2.945M8 3.935V5.5A2.5 2.5 0 0010.5 8h.5a2 2 0 012 2 2 2 0 104 0 2 2 0 012-2h1.064M15 20.488V18a2 2 0 012-2h3.064" />
                ),
                title: "Global Reach",
                text: "Serving congregations across the UK, Ireland, and the Philippines — wherever you are, you're part of the family.",
              },
            ].map((item) => (
              <div key={item.title} className="flex gap-4">
                <div className="w-10 h-10 shrink-0 rounded-full bg-primary/10 flex items-center justify-center">
                  <svg className="w-5 h-5 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    {item.icon}
                  </svg>
                </div>
                <div>
                  <h4 className="font-semibold text-neutral-800">{item.title}</h4>
                  <p className="text-neutral-600 text-sm mt-1">{item.text}</p>
                </div>
              </div>
            ))}

            <a
              href="https://www.youtube.com/@PagasaCentre"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 px-6 py-3 bg-ink text-white font-semibold rounded-md hover:bg-ink-secondary transition-colors"
            >
              Watch on YouTube
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                  d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
              </svg>
            </a>
          </div>
        </div>
      </div>
    </section>
  );
}

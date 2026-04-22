export default function G12Vision() {
  return (
    <section className="bg-ink text-white" id="g12-vision">
      <div className="max-w-7xl mx-auto px-8 sm:px-14 lg:px-20 py-20 lg:py-28">
        <div className="w-14 h-1 bg-primary mb-7" />

        <div className="grid lg:grid-cols-2 gap-16 items-start">
          {/* Heading */}
          <div>
            <h2 className="text-4xl sm:text-5xl font-extrabold leading-tight mb-6">
              The G12 Vision
            </h2>
            <p className="text-white/70 text-base leading-relaxed">
              A discipleship model built on the pattern Jesus set with His twelve disciples —
              each believer mentoring twelve, who in turn mentor twelve more.
            </p>
          </div>

          {/* Body */}
          <div className="space-y-5 text-white/80 text-base leading-relaxed">
            <p>
              The G12 vision — Government of 12 — was conceived by Pastor César
              Castellanos in 1983. Beginning with just eight members in Bogotá,
              Colombia, his church grew to over 150,000 through this structure of
              intentional discipleship.
            </p>
            <p>
              At Pag-Asa Centre, we embrace this vision because it reflects the
              heart of the Great Commission: making disciples who make disciples.
              Every member is both a disciple and a leader, helping others
              experience first-hand the transformative power of Jesus&apos; love.
            </p>
            <p>
              Through cells, leadership development, and community, we are
              building a church that multiplies — not just grows.
            </p>
          </div>
        </div>

        {/* Stat callout */}
        <div className="mt-16 border-t border-white/10 pt-12 grid sm:grid-cols-3 gap-10 text-center">
          {[
            { figure: "2007", label: "Year established" },
            { figure: "G12", label: "Government of Twelve" },
            { figure: "9+", label: "Locations worldwide" },
          ].map(({ figure, label }) => (
            <div key={label}>
              <p className="text-5xl font-extrabold text-primary mb-2">{figure}</p>
              <p className="text-xs font-semibold uppercase tracking-widest text-white/60">{label}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

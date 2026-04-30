export default function AboutVideo() {
  return (
    <section className="py-20 bg-surface">
      <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-10">
          <div className="w-14 h-1 bg-neutral-800 mb-7 mx-auto" />
          <h2 className="text-3xl sm:text-4xl font-extrabold text-neutral-900 mb-4">
            Watch our story
          </h2>
          <p className="text-neutral-600 max-w-2xl mx-auto">
            Get a glimpse of who we are, what we believe, and how God has been
            moving in our community.
          </p>
        </div>

        <div className="relative w-full aspect-video rounded-lg overflow-hidden shadow-lg">
          <iframe
            src="https://www.youtube.com/embed/R1FKgMnBJNw"
            title="Pag-Asa Centre"
            allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
            allowFullScreen
            className="absolute inset-0 w-full h-full"
          />
        </div>
      </div>
    </section>
  );
}

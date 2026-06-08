import Image from "next/image";
import Link from "next/link";
import { events, CAMP_REGISTRATION_CLOSED_LABEL } from "@/lib/events";
import { googleMapsUrl } from "@/lib/maps";
import type { Event } from "@/types";

function CalendarIcon() {
  return (
    <svg
      className="w-4 h-4"
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"
      />
    </svg>
  );
}

function ClockIcon() {
  return (
    <svg
      className="w-4 h-4"
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
      />
    </svg>
  );
}

function MapPinIcon() {
  return (
    <svg
      className="w-4 h-4 shrink-0 mt-0.5"
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M17.657 16.657L13.414 20.9a2 2 0 01-2.828 0l-4.244-4.243a8 8 0 1111.314 0z"
      />
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M15 11a3 3 0 11-6 0 3 3 0 016 0z"
      />
    </svg>
  );
}

function ExternalLinkIcon() {
  return (
    <svg
      className="inline-block w-3 h-3 ml-0.5 align-baseline opacity-60 shrink-0"
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
      />
    </svg>
  );
}

function EventCard({
  event,
  registrationsOpen,
}: {
  event: Event;
  registrationsOpen: boolean;
}) {
  // The Summer Camp card is the only CTA that points at the registration form.
  const isCampRegistration = event.cta?.href === "/camp/register";
  const showClosedCta = isCampRegistration && !registrationsOpen;
  return (
    <article className="group flex flex-col bg-white rounded-xl shadow-sm border border-neutral-300 overflow-hidden hover:shadow-md hover:border-primary/30 transition-all">
      <div className="relative h-48 bg-neutral-200 overflow-hidden">
        {event.imageUrl ? (
          <Image
            src={event.imageUrl}
            alt={event.title}
            fill
            sizes="(min-width: 1024px) 33vw, (min-width: 640px) 50vw, 100vw"
            className="object-cover group-hover:scale-105 transition-transform duration-500"
          />
        ) : (
          <div className="absolute inset-0 flex items-center justify-center">
            <span className="text-neutral-400 text-5xl font-bold">
              {event.title[0]}
            </span>
          </div>
        )}
        <div className="absolute top-4 left-4 inline-flex items-center gap-2 px-3 py-1.5 bg-primary text-white text-xs font-bold uppercase tracking-widest">
          <CalendarIcon />
          {event.date}
        </div>
      </div>

      <div className="flex-1 flex flex-col p-6">
        <div className="flex items-center gap-2 text-primary text-sm font-semibold mb-3">
          <ClockIcon />
          <span>{event.time}</span>
        </div>
        <h3 className="text-xl font-bold text-neutral-900 mb-3 leading-tight group-hover:text-primary transition-colors">
          {event.title}
        </h3>
        {event.description && (
          <p className="text-neutral-600 text-sm leading-relaxed mb-4">
            {event.description}
          </p>
        )}
        {event.location && (
          <a
            href={googleMapsUrl(event.location)}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={`Open ${event.location} in Google Maps`}
            className="flex items-start gap-2 text-neutral-500 text-sm leading-relaxed mb-4 hover:text-primary hover:underline transition-colors"
          >
            <MapPinIcon />
            <span>
              {event.location}
              <ExternalLinkIcon />
            </span>
          </a>
        )}
        <div className="flex-1" />
        {event.cta &&
          (showClosedCta ? (
            <span className="inline-flex items-center justify-center px-5 py-2.5 bg-neutral-200 text-neutral-600 text-xs uppercase tracking-widest font-bold self-start cursor-default">
              {CAMP_REGISTRATION_CLOSED_LABEL}
            </span>
          ) : (
            <Link
              href={event.cta.href}
              className="inline-flex items-center justify-center px-5 py-2.5 bg-primary text-white text-xs uppercase tracking-widest font-bold hover:bg-primary-dark transition-colors self-start"
            >
              {event.cta.label}
            </Link>
          ))}
      </div>
    </article>
  );
}

export default function EventsGrid({
  registrationsOpen = true,
}: {
  registrationsOpen?: boolean;
}) {
  return (
    <section id="events" className="py-20 lg:py-24 bg-surface">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-14">
          <p className="text-primary uppercase tracking-widest text-sm font-semibold mb-2">
            Upcoming Events
          </p>
          <h2 className="text-3xl sm:text-4xl font-bold text-neutral-800">
            Mark your calendar
          </h2>
          <p className="mt-4 text-neutral-600 max-w-xl mx-auto">
            Stay connected and informed about our vibrant community.
          </p>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
          {events.map((e) => (
            <EventCard
              key={e.id}
              event={e}
              registrationsOpen={registrationsOpen}
            />
          ))}
        </div>
      </div>
    </section>
  );
}

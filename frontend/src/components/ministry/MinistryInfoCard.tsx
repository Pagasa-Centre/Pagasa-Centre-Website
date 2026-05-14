import Image from "next/image";
import type { Ministry } from "@/types";

function CalendarIcon() {
  return (
    <svg
      className="w-5 h-5 text-primary"
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
      className="w-5 h-5 text-primary"
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

function LocationIcon() {
  return (
    <svg
      className="w-5 h-5 text-primary"
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M17.657 16.657L13.414 20.9a2 2 0 01-2.828 0l-4.244-4.243a8 8 0 1111.314 0zM15 11a3 3 0 11-6 0 3 3 0 016 0z"
      />
    </svg>
  );
}

function LeaderAvatar({ name, imageUrl }: { name: string; imageUrl?: string }) {
  if (imageUrl) {
    return (
      <Image
        src={imageUrl}
        alt={name}
        width={40}
        height={40}
        className="w-10 h-10 rounded-full object-cover shrink-0"
      />
    );
  }
  const initials = name
    .split(" ")
    .map((n) => n[0])
    .slice(0, 2)
    .join("")
    .toUpperCase();
  return (
    <div className="w-10 h-10 rounded-full bg-primary/15 text-primary flex items-center justify-center text-sm font-bold shrink-0">
      {initials}
    </div>
  );
}

export default function MinistryInfoCard({ ministry }: { ministry: Ministry }) {
  return (
    <div className="bg-white rounded-xl shadow-lg border border-neutral-200 p-7 sm:p-8 w-full">
      <div className="space-y-5">
        <div>
          <p className="text-neutral-500 text-xs uppercase tracking-widest font-semibold mb-2">
            Ministry Day
          </p>
          <div className="flex items-center gap-3">
            <CalendarIcon />
            <span className="font-bold text-neutral-900">{ministry.day}</span>
          </div>
        </div>

        <div className="border-t border-neutral-200" />

        <div>
          <p className="text-neutral-500 text-xs uppercase tracking-widest font-semibold mb-2">
            Ministry Hour
          </p>
          <div className="flex items-center gap-3">
            <ClockIcon />
            <span className="font-bold text-neutral-900">{ministry.time}</span>
          </div>
        </div>

        {ministry.location && (
          <>
            <div className="border-t border-neutral-200" />
            <div>
              <p className="text-neutral-500 text-xs uppercase tracking-widest font-semibold mb-2">
                Ministry Location
              </p>
              <div className="flex items-start gap-3">
                <span className="mt-0.5">
                  <LocationIcon />
                </span>
                <span className="font-bold text-neutral-900">
                  {ministry.location}
                </span>
              </div>
            </div>
          </>
        )}

        {ministry.leaders && ministry.leaders.length > 0 && (
          <>
            <div className="border-t border-neutral-200" />
            <div>
              <p className="text-neutral-500 text-xs uppercase tracking-widest font-semibold mb-3">
                Ministry {ministry.leaders.length === 1 ? "Leader" : "Leaders"}
              </p>
              <ul className="space-y-3">
                {ministry.leaders.map((leader) => (
                  <li key={leader.name} className="flex items-center gap-3">
                    <LeaderAvatar name={leader.name} imageUrl={leader.imageUrl} />
                    <span className="font-bold text-neutral-900">
                      {leader.name}
                    </span>
                  </li>
                ))}
              </ul>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

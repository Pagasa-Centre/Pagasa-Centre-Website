import type { Event } from "@/types";

// TODO: when real photos exist in /public, set imageUrl on each event below.
// Suggested filenames: /event-kci-womens.jpg, /event-g12-uk.jpg,
// /event-pc-anniversary.jpg, /event-pc-summer-camp.jpg,
// /event-pc-christmas.jpg.
export const events: Event[] = [
  {
    id: "kci-womens-conference",
    title: "Be Fruitful and Multiply — KCI Women's Conference",
    date: "April 17–18th",
    time: "Fri: 7:00–9:00PM, Sat: 9:00AM–4:00PM",
    description:
      "Join us for this special Women's Conference hosted by Senior Pastor Adriana Richards of King's Church International. Expect guest speakers, worship and ministry, fellowship, teaching, and so much more. Adults: £50 | Youth (15–18): £25.",
    location: "Braywick Leisure Centre, Braywick Rd, Maidenhead, SL6 1BN",
    cta: { label: "Register now", href: "#" },
  },
  {
    id: "g12-uk-conference",
    title: "G12 UK Conference: Go and Make Disciples of All Nations",
    date: "May 29–30th",
    time: "TBC",
    description:
      "A powerful opportunity for people across the UK to gather for worship, teaching, inspiring talks, interviews, and more — as we see lives transformed by the love and power of Jesus Christ.",
    location: "Braywick Leisure Centre, Braywick Rd, Maidenhead, SL6 1BN",
    cta: { label: "More info", href: "#" },
  },
  {
    id: "pc-19th-anniversary",
    title: "PC 19yrs Anniversary",
    date: "July 19th",
    time: "TBC",
  },
  {
    id: "pc-summer-camp",
    title: "PC Summer Camp",
    date: "August 10–14th",
    time: "TBC",
  },
  {
    id: "pc-christmas-party",
    title: "PC Christmas Party",
    date: "December 20th",
    time: "TBC",
  },
];

export function getEventById(id: string): Event | undefined {
  return events.find((e) => e.id === id);
}

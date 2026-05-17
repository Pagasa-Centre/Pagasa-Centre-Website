import type { Event } from "@/types";

// TODO: when real photos exist in /public, set imageUrl on each event below.
// Suggested filenames: /event-kci-womens.jpg, /event-g12-uk.jpg,
// /event-pc-anniversary.jpg, /event-pc-summer-camp.jpg,
// /event-pc-christmas.jpg.
export const events: Event[] = [
  {
    id: "g12-uk-conference",
    title: "G12 UK Conference: Go and Make Disciples of All Nations",
    date: "May 29–30th",
    time: "May 29–30th",
    description:
      "A powerful opportunity for people across the UK to gather for worship, teaching, inspiring talks, interviews, and more — as we see lives transformed by the love and power of Jesus Christ.",
    location: "Braywick Leisure Centre, Braywick Rd, Maidenhead, SL6 1BN",
    cta: { label: "More info", href: "#" },
    imageUrl: "/event-g12-uk.png",
  },
  {
    id: "pc-19th-anniversary",
    title: "PC 19yrs Anniversary",
    date: "July 19th",
    time: "2:00 PM onwards",
    description:
      "Celebrate with us! Join the Pag-Asa Centre family as we mark our 19th anniversary together.",
    location: "Jo Richardson Community School, Dagenham, RM9 4UN",
    imageUrl: "/event-pc-anniversary.png",
  },
  {
    id: "pc-summer-camp",
    title: "PC Summer Camp 2026",
    date: "August 10–14th",
    time: "Save the date!",
    description:
      "A week of worship, teaching, fellowship, and fun together as a church family. Open to all members — full-week stays or day passes available.",
    location: "Evesham, Worcestershire",
    imageUrl: "/event-pc-summer-camp.png",
    cta: { label: "Register now", href: "/camp/register" },
  },
  {
    id: "pc-christmas-party",
    title: "PC Christmas Party",
    date: "December 20th",
    time: "TBC",
    description:
      "'Tis the season! Join the Pag-Asa Centre family for our Christmas Party.",
    location: "Jo Richardson Community School, Dagenham, RM9 4UN",
    imageUrl: "/event-pc-christmas.png",
  },
];

export function getEventById(id: string): Event | undefined {
  return events.find((e) => e.id === id);
}

import type { Ministry } from "@/types";

// TODO: when real photos exist in /public, set imageUrl on each ministry below.
// Suggested filenames: /ministry-sunday.jpg, /ministry-wildsons.jpg,
// /ministry-production.jpg, /ministry-children.jpg, /ministry-media.jpg,
// /ministry-creative.jpg, /ministry-music.jpg, /ministry-ushering.jpg.
export const ministries: Ministry[] = [
  {
    id: "sunday-celebration",
    name: "Sunday Cell Celebration",
    day: "Sunday",
    time: "2:00 PM",
    description:
      "Join us every Sunday at 2PM for the uplifting Pag-Asa Centre Celebration!",
  },
  {
    id: "wildsons",
    name: "Wildsons",
    day: "Friday",
    time: "6:30 PM",
    description:
      "To all youth, join us every Friday at 6:30 PM for Wildsons — a space built for the next generation.",
  },
  {
    id: "production-team",
    name: "Production Team",
    day: "Sunday",
    time: "2:00 PM",
    description:
      "The Production ministry transports, assembles, and stores the church's assets so every service runs smoothly.",
  },
  {
    id: "childrens-ministry",
    name: "Children's Ministry",
    day: "Sunday",
    time: "3:00 PM",
    description:
      "Nurturing the spiritual growth of the next generation and helping them discover the love of Jesus Christ.",
  },
  {
    id: "media-ministry",
    name: "Media Ministry",
    day: "Sunday",
    time: "2:00 PM",
    description:
      "We focus on using media to spread the word of God and reach communities beyond our walls.",
  },
  {
    id: "creative-arts",
    name: "Creative Arts",
    day: "Sunday",
    time: "12:30 PM",
    description:
      "Our Creative Arts Ministry is all about the beautiful fusion of faith and artistic expression.",
  },
  {
    id: "music-ministry",
    name: "Music Ministry",
    day: "Saturday",
    time: "9:00 AM",
    description:
      "Genuine worship goes beyond performance. We lead the church into encountering God through music.",
  },
  {
    id: "ushering-security",
    name: "Ushering & Security",
    day: "Sunday",
    time: "2:00 PM",
    description:
      "Ushers are the first representatives of Jesus Christ that people meet at a worship service.",
  },
];

export const featuredMinistryIds = [
  "sunday-celebration",
  "production-team",
  "childrens-ministry",
  "media-ministry",
  "wildsons",
] as const;

export const featuredMinistries: Ministry[] = featuredMinistryIds
  .map((id) => ministries.find((m) => m.id === id))
  .filter((m): m is Ministry => Boolean(m));

export function getMinistryById(id: string): Ministry | undefined {
  return ministries.find((m) => m.id === id);
}

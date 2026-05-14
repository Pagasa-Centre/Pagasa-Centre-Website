import type { Ministry } from "@/types";

export const ministries: Ministry[] = [
  {
    id: "sunday-celebration",
    name: "Sunday Cell Celebration",
    day: "Sunday",
    time: "2:00 PM",
    description:
      "Join us every Sunday at 2PM for the uplifting Pag-Asa Centre Celebration!",
    imageUrl: "/ministry-sunday.jpg",
  },
  {
    id: "wildsons",
    name: "Wildsons",
    day: "Friday",
    time: "6:30 PM",
    description:
      "To all youth, join us every Friday at 6:30 PM for Wildsons — a space built for the next generation.",
    imageUrl: "/ministry-wildsons.jpg",
  },
  {
    id: "production-team",
    name: "Production Team",
    day: "Sunday",
    time: "2:00 PM",
    description:
      "The Production ministry is responsible for transporting, assembling, and storing the church's assets and equipment. This ministry involves some lifting and coordination however it is so rewarding to see that all the equipment are working and in its right place.",
    imageUrl: "/ministry-production.jpg",
    location: "Jo Richardson Community School",
    leaders: [{ name: "Kenneth Camposano" }, { name: "Ash Ramanah" }],
    about: [
      "Our church has a dedicated Production Team that is responsible for creating an atmosphere of worship and celebration through decorations and visual effects. The team is comprised of talented volunteers and designers who are passionate about using their gifts to enhance the worship experience. They also help provide decorations and visual effects for special events throughout the year. They are an integral part of our church's ministry, helping to create a sense of joy and celebration in everything that we do.",
      "If you are interested in joining our Production Team or would like more information, please don't hesitate to contact your cell leader or click \"Get Involved\" below. We would love to have you join our team and help us create beautiful and inspiring worship experiences for our congregation.",
    ],
  },
  {
    id: "childrens-ministry",
    name: "Children's Ministry",
    day: "Sunday",
    time: "3:00 PM",
    description:
      "Nurturing the spiritual growth of the next generation and helping them discover the love of Jesus Christ.",
    imageUrl: "/ministry-children.jpg",
  },
  {
    id: "media-ministry",
    name: "Media Ministry",
    day: "Sunday",
    time: "2:00 PM",
    description:
      "We focus on using media to spread the word of God and reach communities beyond our walls.",
    imageUrl: "/ministry-media.jpg",
  },
  {
    id: "creative-arts",
    name: "Creative Arts",
    day: "Sunday",
    time: "12:30 PM",
    description:
      "Our Creative Arts Ministry is all about the beautiful fusion of faith and artistic expression.",
    imageUrl: "/ministry-creative.jpg",
  },
  {
    id: "music-ministry",
    name: "Music Ministry",
    day: "Saturday",
    time: "9:00 AM",
    description:
      "Genuine worship goes beyond performance. We lead the church into encountering God through music.",
    imageUrl: "/ministry-music.jpg",
  },
  {
    id: "ushering-security",
    name: "Ushering & Security",
    day: "Sunday",
    time: "2:00 PM",
    description:
      "Ushers are the first representatives of Jesus Christ that people meet at a worship service.",
    imageUrl: "/ministry-ushering.jpg",
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

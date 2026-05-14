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
    aboutSections: [
      {
        body: [
          "Our church has a dedicated Production Team that is responsible for creating an atmosphere of worship and celebration through decorations and visual effects. The team is comprised of talented volunteers and designers who are passionate about using their gifts to enhance the worship experience. They also help provide decorations and visual effects for special events throughout the year. They are an integral part of our church's ministry, helping to create a sense of joy and celebration in everything that we do.",
          "If you are interested in joining our Production Team or would like more information, please don't hesitate to contact your cell leader or click \"Get Involved\" below. We would love to have you join our team and help us create beautiful and inspiring worship experiences for our congregation.",
        ],
      },
    ],
  },
  {
    id: "childrens-ministry",
    name: "Children's Ministry",
    day: "Sunday",
    time: "3:00 PM",
    description:
      "Our Children's Ministry is dedicated to nurturing their spiritual growth and helping them discover the love of Jesus Christ.",
    imageUrl: "/ministry-children.jpg",
    location: "Jo Richardson Community School",
    leaders: [{ name: "Regina Bulaong" }, { name: "Ash Ramanah" }],
    aboutSections: [
      {
        body: [
          "At Pag-Asa Centre, we believe that children are a precious gift from God, and our Children's Ministry is dedicated to nurturing their spiritual growth and helping them discover the love of Jesus Christ. We are inspired by Jesus' own words when He emphasized the importance of children in the Kingdom of God, declaring, \"Let the little children come to me, and do not hinder them, for the kingdom of heaven belongs to such as these\" (Matthew 19:14, NIV).",
        ],
      },
      {
        heading: "Passionate Volunteers, Dedicated to Your Children's Faith",
        body: [
          "Our Children's Ministry, led by a team of dedicated and passionate volunteers, is committed to creating a safe, loving, and engaging environment where children can learn, grow, and develop a strong foundation of faith. Our volunteers are enthusiastic about guiding your little ones in their journey to know, love, and follow Jesus.",
        ],
      },
      {
        heading: "Join Our Ministry Family",
        body: [
          "If you share our passion for nurturing the spiritual growth of children and helping them build a lasting relationship with God, we invite you to become a part of our Children's Ministry team. Your unique talents and gifts can make a meaningful difference in the lives of our young ones.",
        ],
      },
      {
        heading: "Raising the Next Godly Generation, Together",
        body: [
          "We firmly believe that by investing in our children today, we are shaping the future of our church and community. Together, as a faith community, we can play a vital role in raising the next godly generation. Our mission is to equip children with the knowledge, values, and faith that will guide them throughout their lives.",
        ],
      },
      {
        heading: "Join Us Today",
        body: [
          "Whether you're a parent looking to involve your child in our ministry or an individual with a heart to serve the Lord through the Children's Ministry, we welcome you with open arms. Come and join us as we embark on this inspiring journey of faith, hope, and love with the youngest members of our church family.",
          "Together, we can make a lasting impact in the lives of our children and help them shine as lights in the world, sharing God's love and grace with everyone they meet.",
          "Join our Children's Ministry by clicking the \"Get involved\" button and filling in the form.",
        ],
      },
    ],
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

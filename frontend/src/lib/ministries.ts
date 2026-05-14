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
      "The Media Ministry is the Church's evangelistic extension that focuses on using media to spread the word of God.",
    imageUrl: "/ministry-media.jpg",
    location: "Jo Richardson Community School",
    leaders: [{ name: "Gian Ambat" }, { name: "Ash Ramanah" }],
    aboutSections: [
      {
        body: [
          "The Media Ministry is the Church's evangelistic extension that focuses on using media to spread the word of God. The technical assistance needed for our worship services, archived teachings, sermon messages, and other events will be taught. You will have the chance to use various forms of technology, media outlets, and social media platforms to effectively share the Good News of Jesus Christ with people all over the world if you choose to serve in the team!",
          "You have lots of options for serving in the media ministry. The Pag-Asa Centre Media team is here to assist you in learning what you'll need to know while having fun!",
        ],
      },
      {
        heading: "How to get involved in the ministry?",
        body: [
          "Please contact Pastor Gian or your cell leader if you want to be a member. You can also click \"Get Involved\" below and fill in the form!",
        ],
      },
      {
        heading: "Ministry activities and schedule",
        body: ["Media Ministry serves every Sunday from 2pm-5pm. Activities involve:"],
        bullets: ["Photography", "Videography", "Editing", "Projection"],
      },
    ],
  },
  {
    id: "creative-arts",
    name: "Creative Arts",
    day: "Sunday",
    time: "12:30 PM",
    description:
      "Our Creative Arts Ministry is dedicated to creating a vibrant space where these gifts can flourish, and where we can collectively use them to glorify the Lord.",
    imageUrl: "/ministry-creative.jpg",
    location: "Jo Richardson - Drama Studio 2",
    leaders: [{ name: "Nathan Gordon" }, { name: "Ash Ramanah" }],
    aboutSections: [
      {
        body: [
          "At Pag-Asa Centre, we believe that every individual is uniquely gifted by God with an array of talents and abilities. Our Creative Arts Ministry is dedicated to creating a vibrant space where these gifts can flourish, and where we can collectively use them to glorify the Lord.",
        ],
      },
      {
        heading:
          "Discover Your Creative Calling at Pagasa Centre: Creative Arts Ministry",
        body: [
          "\"God has given each of you a gift from his great variety of spiritual gifts. Use them well to serve one another.\" — 1 Peter 4:10 (NLT)",
        ],
      },
      {
        heading: "The Power of Creative Expression",
        body: [
          "Our Creative Arts Ministry is all about the beautiful fusion of faith and artistic expression. We find inspiration in the words of 1 Peter 4:10, which remind us that our spiritual gifts are meant to be shared to serve one another and bring glory to God. Through the captivating mediums of dance and acting, we aim to spread joy, love, and our heartfelt praise through the powerful language of movement.",
        ],
      },
      {
        heading: "Meet Our Visionary Leader",
        body: [
          "Under the expert guidance of our visionary leader, Nathan Gordon, a seasoned professional dancer and choreographer, our ministry gathers weekly to craft and refine church-wide productions that inspire, uplift, and engage. Nathan's passion for the arts and unwavering faith in God's grace infuse our ministry with boundless creativity and enthusiasm.",
        ],
      },
      {
        heading: "Open Doors, Open Hearts",
        body: [
          "Whether you have been blessed with a creative gift that's ready to shine or if you're simply intrigued and eager to explore your artistic side, the doors of our Creative Arts Ministry are wide open to all. We believe that creativity knows no bounds, and everyone is welcome to join us on this journey of discovery, expression, and faith.",
        ],
      },
      {
        heading: "Join Us Today",
        body: [
          "Come and be a part of a community where your creativity is celebrated, and your artistic journey is nurtured. At Pag-Asa Centre's Creative Arts Ministry, we are more than a team; we are a family united by our love for God and our passion for creativity. Together, we seek to create meaningful, impactful, and spiritually uplifting productions that touch hearts and inspire souls.",
        ],
      },
      {
        heading: "Discover, Create, Worship",
        body: [
          "Unlock the door to your creative potential, and let your talents shine as a vibrant part of our Creative Arts Ministry. Together, we will illuminate the world with the message of God's love and grace through the universal language of art and expression.",
          "Join us and let your creative spirit soar at Pag-Asa Centre's Creative Arts Ministry by clicking \"Get Involved\" and filling in the form.",
        ],
      },
      {
        heading: "Ministry activities and schedule",
        body: [
          "Day: Sundays",
          "Time: 12:30 — 13:30",
          "Location: Jo Richardson — Drama Studio 2",
        ],
      },
    ],
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

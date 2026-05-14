export interface MinistryLeader {
  name: string;
  imageUrl?: string;
}

export interface MinistryAboutSubsection {
  heading: string;
  body: string[];
}

export interface MinistryAboutSection {
  heading?: string;
  body: string[];
  bullets?: string[];
  subsections?: MinistryAboutSubsection[];
}

export interface Ministry {
  id: string;
  name: string;
  day: string;
  time: string;
  description: string;
  imageUrl?: string;
  location?: string;
  aboutSections?: MinistryAboutSection[];
  leaders?: MinistryLeader[];
  getInvolvedHref?: string;
}

export interface Location {
  id: string;
  city: string;
  country: string;
  address: string;
  venue?: string;
}

export interface Testimonial {
  id: string;
  name: string;
  location: string;
  quote: string;
  avatarUrl?: string;
}

export interface Leader {
  id: string;
  name: string;
  title: string;
  bio: string;
  imageUrl?: string;
}

export interface Belief {
  number: number;
  title: string;
  description: string;
  scripture: string;
}

export interface Event {
  id: string;
  title: string;
  date: string;
  time: string;
  description?: string;
  location?: string;
  cta?: { label: string; href: string };
  imageUrl?: string;
}

export interface LocationDetail {
  id: string;
  name: string;
  venue?: string;
  address: string;
  schedule: string;
  phone?: string;
  email?: string;
  imageUrl?: string;
}

// Future: returned from Go API once auth is added
export interface User {
  id: string;
  email: string;
  name: string;
  role: "member" | "leader" | "admin";
  createdAt: string;
}

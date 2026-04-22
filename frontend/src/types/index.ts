export interface Ministry {
  id: string;
  name: string;
  day: string;
  time: string;
  description: string;
  imageUrl?: string;
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

// Future: returned from Go API once auth is added
export interface User {
  id: string;
  email: string;
  name: string;
  role: "member" | "leader" | "admin";
  createdAt: string;
}

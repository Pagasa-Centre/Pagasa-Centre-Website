import type { Metadata } from "next";
import { Geist } from "next/font/google";
import "./globals.css";

const geist = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Home - Pagasa Centre",
  description:
    "Pag-Asa Centre is a nondenominational, charity church that has been established by God in the year 2008. As a church, we have a passion for God's presence, a deep craving to reach the lost, possess sincere integrity, have spirit-filled faith, down to earth humility and brokenness.",
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={`${geist.variable} scroll-smooth`}>
      <body className="min-h-screen flex flex-col antialiased">{children}</body>
    </html>
  );
}

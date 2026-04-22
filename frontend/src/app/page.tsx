import Navbar from "@/components/layout/Navbar";
import Footer from "@/components/layout/Footer";
import Hero from "@/components/home/Hero";
import Ministries from "@/components/home/Ministries";
import About from "@/components/home/About";
import Leadership from "@/components/home/Leadership";
import PrayerChain from "@/components/home/PrayerChain";
import Sermons from "@/components/home/Sermons";
import Testimonials from "@/components/home/Testimonials";
import Locations from "@/components/home/Locations";
import Newsletter from "@/components/home/Newsletter";

export default function HomePage() {
  return (
    <>
      <Navbar />
      <main>
        <Hero />
        <Ministries />
        <About />
        <Leadership />
        <PrayerChain />
        <Sermons />
        <Testimonials />
        <Locations />
        <Newsletter />
      </main>
      <Footer />
    </>
  );
}

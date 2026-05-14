import type { Metadata } from "next";
import { notFound } from "next/navigation";

import Navbar from "@/components/layout/Navbar";
import Footer from "@/components/layout/Footer";
import Newsletter from "@/components/home/Newsletter";
import MinistryDetail from "@/components/ministry/MinistryDetail";
import BrowseMoreMinistries from "@/components/ministry/BrowseMoreMinistries";

import { ministries, getMinistryById } from "@/lib/ministries";

type Params = { id: string };

export function generateStaticParams(): Params[] {
  return ministries.map((m) => ({ id: m.id }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<Params>;
}): Promise<Metadata> {
  const { id } = await params;
  const ministry = getMinistryById(id);
  if (!ministry) {
    return { title: "Ministry Not Found - Pagasa Centre" };
  }
  return {
    title: `${ministry.name} - Pagasa Centre`,
    description: ministry.description,
  };
}

export default async function MinistryPage({
  params,
}: {
  params: Promise<Params>;
}) {
  const { id } = await params;
  const ministry = getMinistryById(id);
  if (!ministry) notFound();

  return (
    <>
      <Navbar />
      <main>
        <MinistryDetail ministry={ministry} />
        <BrowseMoreMinistries currentId={ministry.id} />
        <Newsletter />
      </main>
      <Footer />
    </>
  );
}

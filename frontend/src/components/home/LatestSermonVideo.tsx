"use client";

import { useEffect, useState } from "react";
import { sermons } from "@/lib/api";

const FALLBACK_VIDEO_ID = "eVnwI5YWbzU";
const FALLBACK_TITLE = "Latest sermon — Pag-Asa Centre";

export default function LatestSermonVideo() {
  const [videoId, setVideoId] = useState(FALLBACK_VIDEO_ID);
  const [title, setTitle] = useState(FALLBACK_TITLE);

  useEffect(() => {
    let cancelled = false;
    sermons
      .latest()
      .then((s) => {
        if (cancelled) return;
        if (s.videoId) {
          setVideoId(s.videoId);
          setTitle(s.title || FALLBACK_TITLE);
        }
      })
      .catch(() => {
        // Keep fallback embed on 404 or network errors.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="relative aspect-video bg-ink rounded-2xl overflow-hidden shadow-xl">
      <iframe
        src={`https://www.youtube-nocookie.com/embed/${videoId}?rel=0`}
        title={title}
        allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
        allowFullScreen
        referrerPolicy="strict-origin-when-cross-origin"
        className="absolute inset-0 w-full h-full"
      />
    </div>
  );
}

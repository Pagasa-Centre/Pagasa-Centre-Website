// Build a Google Maps search URL for a free-text address/place.
// Uses the documented Maps URL API, which opens the location in the Google
// Maps app on mobile or a new browser tab on desktop.
export function googleMapsUrl(...parts: (string | null | undefined)[]): string {
  const query = parts
    .map((p) => (p ?? "").trim())
    .filter(Boolean)
    .join(", ");
  return `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(query)}`;
}

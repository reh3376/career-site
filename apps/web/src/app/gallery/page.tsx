import type { Metadata } from "next";
import Image from "next/image";
import { redirect } from "next/navigation";

import { listPhotos } from "@/lib/photos";
import { getSessionUser } from "@/lib/session-user";

export const metadata: Metadata = {
  title: "Gallery",
  description: "Professional portraits and on-the-floor work photos, with context.",
};
export const dynamic = "force-dynamic";

// Members-only (FSD route table). The proxy already redirects a
// signed-out visitor; this re-check keeps the page correct if the
// allow-list ever drifts.
export default async function GalleryPage() {
  const me = await getSessionUser();
  if (!me) redirect("/login?next=/gallery");
  const photos = await listPhotos();

  return (
    <div className="mx-auto max-w-6xl px-6 py-16 sm:px-10 sm:py-24">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
        on the floor
      </p>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Gallery.
      </h1>
      <p className="mt-6 max-w-2xl text-lg leading-relaxed text-ink-2">
        Where the work happens: control rooms, columns, tank farms,
        greenfield builds, and the desk the writing comes from. Every
        photo carries its context and the year; nothing here is stock.
      </p>

      {photos.length === 0 ? (
        <p className="mt-12 text-sm text-ink-3">
          No photos have been built yet. Run the gallery build and
          commit the derivatives.
        </p>
      ) : (
        <ul className="mt-14 grid gap-x-8 gap-y-14 sm:grid-cols-2">
          {photos.map((p, i) => (
            <li key={p.slug} className="m-0">
              <figure className="m-0">
                <div className="overflow-hidden border border-line bg-paper-2">
                  <Image
                    src={p.src}
                    alt={p.alt}
                    width={p.width}
                    height={p.height}
                    sizes="(min-width: 640px) 50vw, 100vw"
                    priority={i < 2}
                    unoptimized
                    className="block h-auto w-full"
                  />
                </div>
                <figcaption className="mt-4">
                  <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                    {p.taken ? <>{p.taken}<span className="text-ink-4"> · </span></> : null}
                    {p.title}
                  </p>
                  <p className="mt-2 text-sm leading-relaxed text-ink">{p.caption}</p>
                  {p.context ? (
                    <p className="mt-1 text-sm leading-relaxed text-ink-3">{p.context}</p>
                  ) : null}
                </figcaption>
              </figure>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

import type { Metadata } from "next";
import Image from "next/image";

import { AccessNote } from "@/components/access-note";
import { listPhotos } from "@/lib/photos";
import { getSessionUser } from "@/lib/session-user";

export const metadata: Metadata = {
  title: "Gallery",
  description: "Professional portraits and on-the-floor work photos, with context.",
};
// Branches on the session, so it is rendered per request.
export const dynamic = "force-dynamic";

// One URL, two audiences (decided 2026-09-22). An anonymous visitor
// sees the work photos, which are the evidence that makes the case;
// a member sees those plus the personal ones. Each photo declares
// which it is in its front matter, so the split is content, not code.
export default async function GalleryPage() {
  const me = await getSessionUser();
  const all = await listPhotos();
  const photos = me ? all : all.filter((p) => p.visibility === "public");
  const withheld = all.length - photos.length;

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
        Where the work happens: control rooms, columns, tank farms and
        greenfield builds. Every photo carries its context and the year;
        nothing here is stock.
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

      {!me && withheld > 0 ? <AccessNote variant="gallery" /> : null}
    </div>
  );
}

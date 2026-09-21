import Link from "next/link";

import { listArticles } from "@/lib/articles";

// A three-tile strip surfacing the newest publishable articles on the
// public landing page. Server component, reads the filesystem
// allow-list at request time. Renders nothing when there are zero
// articles, so a stripped-back build doesn't leave an empty section.
export async function RecentWritingStrip() {
  const all = await listArticles();
  if (all.length === 0) return null;
  const recent = all.slice(0, 3);

  return (
    <section
      aria-labelledby="recent-writing-heading"
      className="border-t border-line bg-paper"
    >
      <div className="mx-auto max-w-6xl px-6 py-20 sm:px-10 sm:py-24">
        <div className="flex flex-wrap items-baseline justify-between gap-4">
          <div>
            <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              recent writing
            </p>
            <h2
              className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
              style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
            >
              What I&rsquo;ve been thinking about.
            </h2>
          </div>
          <Link
            href="/articles"
            className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent no-underline transition-colors hover:text-accent-hover"
          >
            All writing →
          </Link>
        </div>

        <ul className="mt-12 grid gap-6 sm:grid-cols-3">
          {recent.map((a) => (
            <li key={a.slug}>
              <Link
                href={`/articles/${a.slug}`}
                className="group block h-full border-t-2 border-line-strong pt-4 no-underline transition-colors hover:border-accent"
              >
                <h3
                  className="font-display text-xl leading-snug text-ink transition-colors group-hover:text-accent"
                  style={{ fontVariationSettings: '"opsz" 40, "SOFT" 50' }}
                >
                  {a.title}
                </h3>
                {a.subtitle ? (
                  <p className="mt-3 text-sm leading-relaxed text-ink-2">
                    {a.subtitle}
                  </p>
                ) : null}
                <p className="mt-4 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                  read →
                </p>
              </Link>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}

import type { Metadata } from "next";
import Link from "next/link";

import { listArticles } from "@/lib/articles";

export const metadata: Metadata = {
  title: "Articles",
  description: "Roger Henley's writing on industrial digital transformation, decision-making, and manufacturing AI.",
};

export default async function ArticlesIndexPage() {
  const articles = await listArticles();
  return (
    <div className="mx-auto max-w-3xl px-6 py-16 sm:px-10 sm:py-24">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
        writing
      </p>
      <h1
        className="font-display mt-4 text-5xl leading-[0.98] tracking-tight text-ink sm:text-6xl"
        style={{ fontVariationSettings: '"opsz" 144, "SOFT" 40' }}
      >
        Articles.
      </h1>
      <p className="mt-6 max-w-2xl text-base leading-relaxed text-ink-2">
        Pieces about the parts of digital transformation that don&rsquo;t
        survive the vendor slides, control-room adoption, decision
        infrastructure, and why the plant historian is a distributed
        database whether IT knows it or not.
      </p>

      {articles.length === 0 ? (
        <p className="mt-12 text-sm text-ink-3">
          Nothing to show yet.
        </p>
      ) : (
        <ul className="mt-14 divide-y divide-line border-y border-line">
          {articles.map((a) => (
            <li key={a.slug} className="py-7">
              <Link
                href={`/articles/${a.slug}`}
                className="group block no-underline"
              >
                <h2
                  className="font-display text-2xl leading-snug text-ink transition-colors group-hover:text-accent sm:text-3xl"
                  style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
                >
                  {a.title}
                </h2>
                {a.subtitle ? (
                  <p className="mt-2 text-base leading-relaxed text-ink-2">
                    {a.subtitle}
                  </p>
                ) : null}
                <p className="mt-3 flex flex-wrap items-baseline gap-x-3 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                  {a.date ? <span>{a.date}</span> : null}
                  {a.date ? <span className="text-ink-4">·</span> : null}
                  <span className="text-accent">read →</span>
                </p>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

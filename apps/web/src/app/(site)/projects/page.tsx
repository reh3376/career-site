import type { Metadata } from "next";
import Link from "next/link";

import { listProjects, STATUS_LABEL } from "@/lib/projects";

// The project catalog (FR-CNT-03).
//
// Name, one line, the owner's role, status, technologies and links,
// which is what the row asks for. The one-line summary is the whole
// catalog entry on purpose: a catalog whose entries are paragraphs is
// a list nobody reads to the end, and the detail page is one click
// away.
//
// Live GitHub metadata (stars, language, last commit) is a different
// requirement, FR-CNT-25, served by the landing page's repo cards.
// This page does not fetch from GitHub: a catalog that cannot render
// when api.github.com is slow is a catalog that is sometimes missing.

export const metadata: Metadata = {
  title: "Projects",
  description:
    "Software built in the open: an industrial data hub, a graph memory substrate for agents, PLC code in version control, and this site.",
  alternates: { canonical: "/projects" },
};

export default async function ProjectsPage() {
  const projects = await listProjects();

  return (
    <div className="mx-auto max-w-3xl px-6 py-24 sm:px-10 sm:py-32">
      <p className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
        built in the open
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Projects.
      </h1>
      <p className="mt-6 max-w-xl text-base leading-relaxed text-ink-2">
        Work that stands on its own, rather than the work inside a job.
        What each one is for, what state it is in, and the code where
        there is code.
      </p>

      {projects.length === 0 ? (
        <p className="mt-10 border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          No projects are published yet.
        </p>
      ) : (
        <ul className="mt-16 divide-y divide-line border-y border-line">
          {projects.map((p) => (
            <li key={p.id} className="py-7">
              <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
                <h2
                  className="font-display text-2xl leading-tight text-ink"
                  style={{ fontVariationSettings: '"opsz" 40, "SOFT" 50' }}
                >
                  <Link
                    href={`/projects/${p.slug}`}
                    className="text-ink no-underline transition-colors hover:text-accent"
                  >
                    {p.title}
                  </Link>
                </h2>
                <p className="font-mono text-[10px] tracking-[0.14em] text-ink-3 uppercase">
                  {STATUS_LABEL[p.status]}
                </p>
              </div>

              <p className="mt-2 text-base leading-relaxed text-ink-2">
                {p.summary}
              </p>

              <p className="mt-2 text-sm text-ink-3">{p.role}</p>

              {p.technologies.length ? (
                <ul className="mt-4 flex flex-wrap gap-2">
                  {p.technologies.map((t) => (
                    <li
                      key={t}
                      className="border border-line bg-paper-2 px-2.5 py-1 font-mono text-[10px] tracking-[0.1em] text-ink-3 uppercase"
                    >
                      {t}
                    </li>
                  ))}
                </ul>
              ) : null}

              <p className="mt-4 flex flex-wrap gap-x-5 gap-y-2 font-mono text-[11px] tracking-[0.14em] uppercase">
                <Link
                  href={`/projects/${p.slug}`}
                  className="text-accent no-underline hover:underline"
                >
                  Read more
                </Link>
                {p.repo ? (
                  <a
                    href={p.repo}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-ink-3 no-underline hover:text-accent"
                  >
                    Source
                  </a>
                ) : null}
              </p>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

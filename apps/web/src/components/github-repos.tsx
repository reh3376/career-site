import Link from "next/link";

import { fetchPublicRepos, type PublicRepo } from "@/lib/github-repos";

// Two presentational surfaces for the public-repo card. The IT
// variant is editorial (rule-divided list); the OT variant is HMI
// (mono, panel, tag column). The caller picks based on getUiMode().
// Both call the same fetch so a rate-limit outage produces a single
// fallback message that reads the same on either surface.

// -------------------------------------------------------------------
// IT — editorial card block
// -------------------------------------------------------------------

export async function GithubReposIt() {
  const repos = await fetchPublicRepos({ limit: 6 });

  return (
    <section
      aria-labelledby="repos-heading"
      className="mx-auto max-w-5xl px-6 py-24 sm:px-10 sm:py-32"
    >
      <div className="max-w-2xl">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
          open source
        </p>
        <h2
          id="repos-heading"
          className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
          style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
        >
          Public repositories.
        </h2>
        <p className="mt-5 text-base leading-relaxed text-ink-2">
          The frameworks and tools I work in the open. If one lines up
          with something you&rsquo;re building, you can request
          contributor access below and I&rsquo;ll add you.
        </p>
      </div>

      {repos && repos.length > 0 ? (
        <ul className="mt-14 divide-y divide-line border-y border-line">
          {repos.map((r) => (
            <li key={r.fullName} className="py-6 sm:py-8">
              <RepoRow repo={r} variant="it" />
            </li>
          ))}
        </ul>
      ) : (
        <p className="mt-10 max-w-xl text-sm text-ink-3">
          The GitHub API isn&rsquo;t returning repositories right now.
          You can browse them directly at{" "}
          <a
            href="https://github.com/reh3376?tab=repositories"
            rel="noopener noreferrer"
            target="_blank"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            github.com/reh3376
          </a>
          .
        </p>
      )}

      <div className="mt-12 flex flex-wrap items-center gap-x-5 gap-y-3">
        <Link
          href="/contact?category=contributor_access"
          className="inline-flex items-center rounded-md bg-accent px-6 py-3 text-sm font-medium text-white no-underline shadow-sm transition-colors hover:bg-accent-hover"
        >
          Request contributor access
        </Link>
        <a
          href="https://github.com/reh3376?tab=repositories"
          rel="noopener noreferrer"
          target="_blank"
          className="text-sm text-ink-2 underline decoration-line decoration-1 underline-offset-4 transition-colors hover:text-accent hover:decoration-accent"
        >
          See all on GitHub
        </a>
      </div>
    </section>
  );
}

// -------------------------------------------------------------------
// OT — HMI panel row
// -------------------------------------------------------------------

export async function GithubReposOt() {
  const repos = await fetchPublicRepos({ limit: 6 });

  return (
    <section
      aria-label="Public repositories"
      className="mx-auto max-w-[1600px] px-4 pb-6 sm:px-6"
    >
      <div className="mb-1 flex items-baseline justify-between font-mono text-[10px] uppercase tracking-[0.14em] text-ink-3">
        <span>PUBLIC REPOS</span>
        <span className="text-ink-4">[GH-1]</span>
      </div>
      <div className="border border-line-strong bg-paper-2">
        {repos && repos.length > 0 ? (
          <>
            <div className="grid grid-cols-[minmax(0,180px)_1fr_60px_120px] gap-x-4 border-b border-line px-4 py-2 font-mono text-[10px] uppercase tracking-[0.14em] text-ink-3">
              <span>TAG</span>
              <span>DESCRIPTION</span>
              <span className="text-right">STARS</span>
              <span className="text-right">LANG</span>
            </div>
            <ul className="divide-y divide-line">
              {repos.map((r) => (
                <li key={r.fullName}>
                  <RepoRow repo={r} variant="ot" />
                </li>
              ))}
            </ul>
          </>
        ) : (
          <div className="p-4 font-mono text-[12px] text-ink-3">
            api.github.com unreachable — direct link{" "}
            <a
              href="https://github.com/reh3376?tab=repositories"
              rel="noopener noreferrer"
              target="_blank"
              className="text-accent underline"
            >
              github.com/reh3376
            </a>
          </div>
        )}
        <div className="border-t border-line px-4 py-3 flex flex-wrap items-center gap-x-6 gap-y-2 font-mono text-[11px]">
          <Link
            href="/contact?category=contributor_access"
            className="inline-flex items-center border border-accent bg-accent px-3 py-1.5 text-white no-underline transition-colors hover:bg-accent-hover"
          >
            [ REQUEST CONTRIBUTOR ACCESS ]
          </Link>
          <a
            href="https://github.com/reh3376?tab=repositories"
            rel="noopener noreferrer"
            target="_blank"
            className="text-ink-2 no-underline hover:text-accent"
          >
            → all on github.com/reh3376
          </a>
        </div>
      </div>
    </section>
  );
}

// -------------------------------------------------------------------
// One row of a repo, two variants
// -------------------------------------------------------------------

function RepoRow({ repo, variant }: { repo: PublicRepo; variant: "it" | "ot" }) {
  const updated = formatWhen(repo.updatedAt);
  if (variant === "ot") {
    return (
      <div className="grid grid-cols-[minmax(0,180px)_1fr_60px_120px] items-baseline gap-x-4 px-4 py-3 font-mono text-[12px] text-ink-2">
        <a
          href={repo.htmlUrl}
          rel="noopener noreferrer"
          target="_blank"
          className="truncate text-ink no-underline hover:text-accent"
        >
          {repo.name}
        </a>
        <span className="min-w-0">
          <span className="block truncate text-ink-2">
            {repo.description || "—"}
          </span>
          <span className="block truncate text-[10px] uppercase tracking-[0.14em] text-ink-3">
            updated {updated}
          </span>
        </span>
        <span className="text-right tabular text-ink">{repo.stars}</span>
        <span className="text-right text-ink-3">{repo.language || "—"}</span>
      </div>
    );
  }
  return (
    <div className="grid gap-3 sm:grid-cols-[minmax(0,220px)_1fr] sm:gap-8">
      <div>
        <a
          href={repo.htmlUrl}
          rel="noopener noreferrer"
          target="_blank"
          className="font-display text-xl leading-tight text-ink no-underline hover:text-accent"
          style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
        >
          {repo.name}
        </a>
        <p className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          {repo.language ? <span>{repo.language}</span> : null}
          <span>{repo.stars} ★</span>
          <span>updated {updated}</span>
        </p>
      </div>
      <p className="max-w-2xl text-base leading-relaxed text-ink-2">
        {repo.description || (
          <span className="italic text-ink-3">
            No description on GitHub yet.
          </span>
        )}
      </p>
    </div>
  );
}

// "updated 2mo ago" / "updated 3d ago" — server-computed against the
// request time so both landing variants render the same string.
function formatWhen(iso: string): string {
  const then = Date.parse(iso);
  if (Number.isNaN(then)) return iso;
  const days = Math.max(1, Math.round((Date.now() - then) / 86_400_000));
  if (days < 7) return `${days}d ago`;
  if (days < 30) return `${Math.round(days / 7)}w ago`;
  if (days < 365) return `${Math.round(days / 30)}mo ago`;
  return `${Math.round(days / 365)}y ago`;
}

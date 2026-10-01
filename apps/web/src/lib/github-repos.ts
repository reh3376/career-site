import "server-only";

// A tiny shape mapped from GitHub REST /users/{user}/repos entries.
// Not every field the API returns is useful for the card, we pick
// exactly what the UI reads so the tsx callsite is legible and the
// serialized RSC payload stays small.
export type PublicRepo = {
  name: string;
  fullName: string;      // "reh3376/net-topo-toolkit"
  description: string | null;
  htmlUrl: string;
  language: string | null;
  stars: number;
  updatedAt: string;     // ISO
  archived: boolean;
  fork: boolean;
};

// Repos we don't want on the card even though they're public. The
// career-site repo is already surfaced everywhere else (footer, "about
// this site" section), so listing it here would be double-mention.
const HIDDEN = new Set(["career-site"]);

// The repositories worth showing a hiring manager, in the order they
// should appear.
//
// This used to be "whatever GitHub updated most recently", which meant
// the cards reshuffled on any commit and showed whatever happened to be
// touched: a Homebrew packaging tap and a concept map outranked the
// work the résumés actually cite. A visitor cannot tell which of those
// is the proof and which is housekeeping.
//
// An allow-list is a curation decision, so it lives in code where it
// can be read and changed deliberately. Anything not named here is not
// shown, and a name that no longer exists is skipped rather than
// leaving a gap.
const FEATURED = [
  "mdemg",
  "forge",
  "acd-l5x-tool-lib",
  "plc-gbt",
  "ann-research",
] as const;

// One GitHub user we're pulling from today. If the roster ever grows
// (org account, etc.), thread the login through as a param.
const GITHUB_USER = "reh3376";

// Cache the API response for an hour. GitHub's REST rate-limit for
// unauthenticated calls is 60/hr per IP; caching keeps us far under
// that even under load. The `next: { revalidate }` option is how
// App Router honors the cache boundary; the response itself flows
// through the standard `fetch` cache.
const REVALIDATE_SECONDS = 60 * 60;

type ApiRepo = {
  name: string;
  full_name: string;
  description: string | null;
  html_url: string;
  language: string | null;
  stargazers_count: number;
  updated_at: string;
  archived: boolean;
  fork: boolean;
  private: boolean;
};

// Fetch the user's public, non-archived, non-fork repos. Returns
// `null` on any error rather than throwing, the caller renders a
// fallback panel so a rate-limit or upstream outage never breaks the
// landing page.
export async function fetchPublicRepos({
  limit = 8,
}: { limit?: number } = {}): Promise<PublicRepo[] | null> {
  try {
    const resp = await fetch(
      `https://api.github.com/users/${GITHUB_USER}/repos?per_page=100&sort=updated`,
      {
        headers: {
          Accept: "application/vnd.github+json",
          "X-GitHub-Api-Version": "2022-11-28",
        },
        next: { revalidate: REVALIDATE_SECONDS },
      },
    );
    if (!resp.ok) return null;
    const raw = (await resp.json()) as ApiRepo[];
    const byName = new Map(
      raw
        .filter(
          (r) => !r.private && !r.fork && !r.archived && !HIDDEN.has(r.name),
        )
        .map((r) => [r.name.toLowerCase(), r] as const),
    );
    // Featured order first. Stars and the updated date still come from
    // the API, so the cards stay live; only which repos appear, and in
    // what order, is decided here.
    return FEATURED.map((n) => byName.get(n.toLowerCase()))
      .filter((r): r is ApiRepo => r !== undefined)
      .slice(0, limit)
      .map((r) => ({
        name: r.name,
        fullName: r.full_name,
        description: r.description,
        htmlUrl: r.html_url,
        language: r.language,
        stars: r.stargazers_count,
        updatedAt: r.updated_at,
        archived: r.archived,
        fork: r.fork,
      }));
  } catch {
    return null;
  }
}

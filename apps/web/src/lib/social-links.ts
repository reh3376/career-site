// Centralised source for the outbound social links surfaced on the
// public site (footer, IT landing, OT landing). URLs come from env
// so a rebrand doesn't require touching multiple components, and so
// an unset link (e.g. LinkedIn on first deploy) hides everywhere
// rather than rendering a broken tile.
//
// Reads use `process.env.NEXT_PUBLIC_*`, inlined at build time by
// Next.js, so these values are baked into the client bundle. Do not
// put anything sensitive here — treat any value as public.

const LINKEDIN_URL_RE = /^https:\/\/(www\.)?linkedin\.com\/in\/[A-Za-z0-9-]+\/?$/;
const GITHUB_URL_RE = /^https:\/\/github\.com\/[A-Za-z0-9._-]+\/?$/;

export type SocialLinks = {
  github?: string;
  githubHandle?: string;
  linkedin?: string;
};

// getSocialLinks returns only the links whose env values look
// well-formed. An invalid value falls back to undefined rather than
// rendering a broken link — a typo on prod is safer that way.
export function getSocialLinks(): SocialLinks {
  const gh = (process.env.NEXT_PUBLIC_GITHUB_URL ?? "https://github.com/reh3376").trim();
  const li = (process.env.NEXT_PUBLIC_LINKEDIN_URL ?? "").trim();

  const out: SocialLinks = {};
  if (GITHUB_URL_RE.test(gh)) {
    out.github = gh;
    // Turn "https://github.com/reh3376" → "github.com/reh3376" for
    // link text so the URL isn't rendered twice.
    out.githubHandle = gh.replace(/^https:\/\//, "").replace(/\/$/, "");
  }
  if (LINKEDIN_URL_RE.test(li)) {
    out.linkedin = li;
  }
  return out;
}

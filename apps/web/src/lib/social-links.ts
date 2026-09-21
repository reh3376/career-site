// Centralised source for the outbound social links surfaced on the
// site (footer, landing pages, member home). URLs come from env so a
// change doesn't touch multiple components, and an unset link hides
// everywhere rather than rendering a broken tile.
//
// Every consumer is a server component, so the values are read at
// request time from the container's environment (LINKEDIN_URL,
// GITHUB_URL set in compose / .env.prod). The NEXT_PUBLIC_* names are
// still honoured as a fallback, but note that Next inlines those at
// image build time, which is why a value set only on the server never
// appeared: the CI build has no owner URLs.
const LINKEDIN_URL_RE = /^https:\/\/(www\.)?linkedin\.com\/in\/[A-Za-z0-9._-]+\/?$/;
const GITHUB_URL_RE = /^https:\/\/github\.com\/[A-Za-z0-9._-]+\/?$/;

export type SocialLinks = {
  github?: string;
  githubHandle?: string;
  linkedin?: string;
  linkedinHandle?: string;
};

// getSocialLinks returns only the links whose values look well-formed.
// An invalid value falls back to undefined rather than rendering a
// broken link; a typo on prod is safer that way.
export function getSocialLinks(): SocialLinks {
  const gh = (
    process.env.GITHUB_URL ??
    process.env.NEXT_PUBLIC_GITHUB_URL ??
    "https://github.com/reh3376"
  ).trim();
  const li = (process.env.LINKEDIN_URL ?? process.env.NEXT_PUBLIC_LINKEDIN_URL ?? "").trim();
  const out: SocialLinks = {};
  if (GITHUB_URL_RE.test(gh)) {
    out.github = gh;
    out.githubHandle = gh.replace(/^https:\/\//, "").replace(/\/$/, "");
  }
  if (LINKEDIN_URL_RE.test(li)) {
    out.linkedin = li;
    out.linkedinHandle = li.replace(/^https:\/\/(www\.)?/, "").replace(/\/$/, "");
  }
  return out;
}

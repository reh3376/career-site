// The site's own identity: origin, social-card defaults, and the person
// the site is about.
//
// One place, because three files used to carry the origin as a literal
// (sitemap, robots, and now the metadata) and a link that renders with
// the wrong host in a LinkedIn preview is not something anyone notices
// until a recruiter sees it.

export const SITE_ORIGIN = "https://rogerhenley.dev";

export const SITE_NAME = "Roger Henley";

// The card image a pasted link renders with. An industrial photograph
// with no identifiable people in it: the link gets shared into places
// this site does not control, so the default image should not put
// anyone else's face there.
export const OG_IMAGE = {
  url: "/images/bubble-tray.jpeg",
  width: 1200,
  height: 900,
  alt: "A distillation column bubble tray on a working plant floor",
};

// Structured data for the landing page.
//
// The point is narrow: a search engine that understands this can show
// the right person for "Roger Henley" rather than guessing between
// namesakes, and sameAs is what ties the site to the profiles a
// recruiter already trusts.
export function personJsonLd(socials: { linkedin?: string; github?: string }) {
  const sameAs = [socials.linkedin, socials.github].filter(Boolean);
  return {
    "@context": "https://schema.org",
    "@type": "Person",
    name: "Roger E. Henley II",
    url: SITE_ORIGIN,
    jobTitle: "VP of Engineering & Technology",
    description:
      "Thirty years running regulated 24/7 industrial systems. Controls, plant operations, industrial data and applied AI.",
    knowsAbout: [
      "Industrial automation",
      "Process control",
      "Model predictive control",
      "IT/OT convergence",
      "Industrial DataOps",
      "Applied AI",
    ],
    ...(sameAs.length > 0 ? { sameAs } : {}),
  };
}

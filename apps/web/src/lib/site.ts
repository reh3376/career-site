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
  // The file's real dimensions, read from the JPEG rather than assumed.
  // Scrapers treat these as the truth and some reject a card whose
  // image does not match what it claimed.
  width: 1800,
  height: 1350,
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

// Per-page share card and canonical URL.
//
// Next merges `metadata` shallowly: a page that sets only `title` and
// `description` inherits the root layout's `openGraph` *object whole*,
// including its title, description and `url`. So every page that did
// not spell out a card pasted into LinkedIn or iMessage as the
// homepage: the same thirty-year-career headline and the same `og:url`,
// whatever page the link actually pointed at.
//
// The canonical is the sharper half of the bug. The root layout
// declares `canonical: "/"`, and inheriting it means each of those
// pages told a crawler it was a duplicate of the homepage, asking to
// have itself dropped from the index. A page nobody can find is a page
// that may as well not have shipped.
//
// Overriding `openGraph` replaces the parent entirely, so the image has
// to be named again or the card silently degrades to text-only; that is
// the kind of detail a helper should carry once rather than four
// callsites remember.
export function pageMetadata(opts: {
  title: string;
  description: string;
  path: `/${string}`;
  /** Pages still being recruited for or not meant to be found by search. */
  noindex?: boolean;
}) {
  const { title, description, path, noindex } = opts;
  return {
    title,
    description,
    alternates: { canonical: path },
    ...(noindex ? { robots: { index: false, follow: false } } : {}),
    openGraph: {
      type: "website" as const,
      siteName: SITE_NAME,
      title,
      description,
      url: path,
      images: [OG_IMAGE],
    },
    twitter: {
      card: "summary_large_image" as const,
      title,
      description,
      images: [OG_IMAGE.url],
    },
  };
}

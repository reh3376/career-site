import type { MetadataRoute } from "next";

import { listArticles } from "@/lib/articles";
import { INDEXABLE_PATHS } from "@/lib/public-routes";

const BASE = "https://rogerhenley.dev";

// Invites crawlers to the public surface: the landing page, the writing,
// the gallery, and the ways in. Member pages are absent here, disallowed
// in robots.txt and served with X-Robots-Tag: noindex by the proxy.
//
// The point is discovery. A recruiter searching a phrase from one of
// these articles should land on it rather than on a LinkedIn repost.
export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const now = new Date();

  const pages: MetadataRoute.Sitemap = INDEXABLE_PATHS.map((p) => ({
    url: `${BASE}${p === "/" ? "" : p}`,
    lastModified: now,
    changeFrequency: p === "/" ? "weekly" : "monthly",
    priority: p === "/" ? 1 : p === "/articles" ? 0.8 : 0.5,
  }));

  let articles: MetadataRoute.Sitemap = [];
  try {
    articles = (await listArticles()).map((a) => ({
      url: `${BASE}/articles/${a.slug}`,
      // `date` is a year ("2026") or an ISO-like string; anything that
      // does not parse falls back to now rather than emitting garbage.
      lastModified: parseDate(a.date) ?? now,
      changeFrequency: "yearly",
      priority: 0.7,
    }));
  } catch {
    // A sitemap missing its articles is better than a build that fails
    // over content it could not read.
  }

  return [...pages, ...articles];
}

function parseDate(raw: string | undefined): Date | undefined {
  if (!raw) return undefined;
  const text = /^\d{4}$/.test(raw) ? `${raw}-01-01` : raw;
  const d = new Date(text);
  return Number.isNaN(d.getTime()) ? undefined : d;
}

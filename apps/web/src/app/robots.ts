import type { MetadataRoute } from "next";

// Public pages are crawlable; everything behind the member gate is not.
// Member routes also carry X-Robots-Tag: noindex from proxy.ts.
export default function robots(): MetadataRoute.Robots {
  return {
    rules: [
      {
        userAgent: "*",
        allow: ["/", "/contact", "/register", "/login", "/privacy", "/terms"],
        disallow: ["/home", "/jd-upload", "/articles", "/gallery", "/settings", "/admin", "/api/"],
      },
    ],
    sitemap: "https://rogerhenley.dev/sitemap.xml",
  };
}

import type { MetadataRoute } from "next";

import { CRAWLER_DISALLOW, INDEXABLE_PATHS } from "@/lib/public-routes";

// Public pages are crawlable; everything behind the member gate is not.
// Member routes also carry X-Robots-Tag: noindex from proxy.ts. The two
// lists come from lib/public-routes.ts so this file cannot fall out of
// step with what the proxy actually lets through.
export default function robots(): MetadataRoute.Robots {
  return {
    rules: [
      {
        userAgent: "*",
        allow: [...INDEXABLE_PATHS],
        disallow: [...CRAWLER_DISALLOW],
      },
    ],
    sitemap: "https://rogerhenley.dev/sitemap.xml",
  };
}

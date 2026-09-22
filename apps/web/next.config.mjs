/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // No "x-powered-by: Next.js" on responses.
  poweredByHeader: false,
  // Build a standalone server bundle so the production Docker image can copy
  // just the .next/standalone directory instead of the full node_modules tree.
  output: "standalone",
  // Pull the markdown article corpus into the standalone bundle so
  // `fs.readFile("content/articles/*.md")` works at runtime. Next's
  // file tracing only follows explicit `import` statements; the
  // articles are read via `fs`, so we tell it to include them
  // manually. See src/lib/articles.ts.
  outputFileTracingIncludes: {
    // The landing page's RecentWritingStrip reads the same corpus,
    // so `/` needs the markdown files traced in too — a missing
    // entry here means the standalone build succeeds but `/`
    // renders nothing because listArticles() sees an empty dir.
    "/": ["./content/articles/**/*.md"],
    "/articles": ["./content/articles/**/*.md"],
    "/articles/[slug]": ["./content/articles/**/*.md"],
    // Gallery front matter + dimensions manifest (derivatives are
    // static files under public/ and need no tracing).
    "/gallery": ["./content/photos/**"],
    "/home": ["./content/articles/**/*.md", "./content/photos/**"],
  },
};

export default nextConfig;

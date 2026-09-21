/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Build a standalone server bundle so the production Docker image can copy
  // just the .next/standalone directory instead of the full node_modules tree.
  output: "standalone",
  // Pull the markdown article corpus into the standalone bundle so
  // `fs.readFile("content/articles/*.md")` works at runtime. Next's
  // file tracing only follows explicit `import` statements; the
  // articles are read via `fs`, so we tell it to include them
  // manually. See src/lib/articles.ts.
  outputFileTracingIncludes: {
    "/articles": ["./content/articles/**/*.md"],
    "/articles/[slug]": ["./content/articles/**/*.md"],
  },
};

export default nextConfig;

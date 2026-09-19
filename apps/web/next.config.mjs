/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Build a standalone server bundle so the production Docker image can copy
  // just the .next/standalone directory instead of the full node_modules tree.
  output: "standalone",
};

export default nextConfig;

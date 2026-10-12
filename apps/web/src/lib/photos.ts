import fs from "node:fs/promises";
import path from "node:path";

import { parseFrontMatter } from "./frontmatter";

// Gallery content (FSD FR-CNT-11). One front-matter file per photo
// under apps/web/content/photos/; derivatives and dimensions come
// from scripts/photos-build.mjs, which is the only path that touches
// originals. Nothing here reads docs/personal.

export type Photo = {
  // Authored, stable across a rename; the slug is derived from the
  // filename. See ArticleSummary.id in lib/articles.ts.
  id: string;
  slug: string;
  title: string;
  caption: string;
  context?: string;
  taken?: string;
  alt: string;
  permission: string;
  heroTracks: string[];
  visibility: "member" | "public";
  order: number;
  width: number;
  height: number;
  src: string; // 1600 px derivative
  srcSmall: string; // 800 px derivative
};

type Manifest = Record<string, { width: number; height: number }>;

const CONTENT_DIR = path.join(process.cwd(), "content", "photos");


export async function listPhotos(): Promise<Photo[]> {
  let files: string[];
  let manifest: Manifest = {};
  try {
    files = (await fs.readdir(CONTENT_DIR)).filter((f) => f.endsWith(".md"));
    manifest = JSON.parse(
      await fs.readFile(path.join(CONTENT_DIR, "manifest.json"), "utf8"),
    ) as Manifest;
  } catch {
    return [];
  }
  const out: Photo[] = [];
  for (const file of files) {
    const slug = file.replace(/\.md$/, "");
    const dims = manifest[slug];
    if (!dims) continue; // not built yet; the script reports why
    const { meta } = parseFrontMatter(
      await fs.readFile(path.join(CONTENT_DIR, file), "utf8"),
    );
    if (!meta.permission || !meta.alt) continue;
    out.push({
      id: meta.id || slug,
      slug,
      title: meta.title ?? slug,
      caption: meta.caption ?? "",
      context: meta.context || undefined,
      taken: meta.taken || undefined,
      alt: meta.alt,
      permission: meta.permission,
      heroTracks: (meta.hero_tracks ?? "")
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean),
      visibility: meta.visibility === "public" ? "public" : "member",
      order: Number(meta.order ?? 1000),
      width: dims.width,
      height: dims.height,
      src: `/images/gallery/${slug}-1600.webp`,
      srcSmall: `/images/gallery/${slug}-800.webp`,
    });
  }
  out.sort((a, b) => a.order - b.order || a.slug.localeCompare(b.slug));
  return out;
}

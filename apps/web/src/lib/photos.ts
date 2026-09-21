import fs from "node:fs/promises";
import path from "node:path";

// Gallery content (FSD FR-CNT-11). One front-matter file per photo
// under apps/web/content/photos/; derivatives and dimensions come
// from scripts/photos-build.mjs, which is the only path that touches
// originals. Nothing here reads docs/personal.

export type Photo = {
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

function parseFrontMatter(raw: string): Record<string, string> {
  const text = raw.replace(/\r\n/g, "\n");
  if (!text.startsWith("---\n")) return {};
  const end = text.indexOf("\n---\n", 4);
  if (end < 0) return {};
  const meta: Record<string, string> = {};
  for (const line of text.slice(4, end).split("\n")) {
    const m = /^([A-Za-z_][A-Za-z0-9_-]*):\s*(.*)$/.exec(line);
    if (!m) continue;
    let v = m[2].trim();
    if (
      v.length >= 2 &&
      ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'")))
    ) {
      v = v.slice(1, -1);
    }
    meta[m[1]] = v;
  }
  return meta;
}

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
    const meta = parseFrontMatter(await fs.readFile(path.join(CONTENT_DIR, file), "utf8"));
    if (!meta.permission || !meta.alt) continue;
    out.push({
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

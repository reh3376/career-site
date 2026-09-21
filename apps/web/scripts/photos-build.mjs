// Builds the gallery derivatives the site serves (FSD FR-CNT-11).
//
// For every content/photos/<slug>.md with front matter, reads the
// original named by `source` (relative to PHOTOS_SOURCE_DIR, default
// ../../docs/personal/images, which is gitignored; or `public/...`
// for an image already committed), applies the EXIF orientation, then
// writes 1600 px and 800 px WebP derivatives with all metadata
// (including GPS) stripped to public/images/gallery/. Originals are
// never copied. Dimensions land in content/photos/manifest.json so the
// page can size <Image> without probing files at request time.
//
// A photo without a recorded `permission` is refused (FSD D-15).
//
//   node scripts/photos-build.mjs            # all photos
//   PHOTOS_SOURCE_DIR=/path node scripts/photos-build.mjs
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import sharp from "sharp";

const here = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(here, "..");
const contentDir = path.join(webRoot, "content", "photos");
const outDir = path.join(webRoot, "public", "images", "gallery");
const sourceDir = process.env.PHOTOS_SOURCE_DIR
  ? path.resolve(process.env.PHOTOS_SOURCE_DIR)
  : path.resolve(webRoot, "..", "..", "docs", "personal", "images");

const WIDTHS = [1600, 800];

function parseFrontMatter(raw) {
  const text = raw.replace(/\r\n/g, "\n");
  if (!text.startsWith("---\n")) return {};
  const end = text.indexOf("\n---\n", 4);
  if (end < 0) return {};
  const meta = {};
  for (const line of text.slice(4, end).split("\n")) {
    const m = /^([A-Za-z_][A-Za-z0-9_-]*):\s*(.*)$/.exec(line);
    if (!m) continue;
    let v = m[2].trim();
    if (v.length >= 2 && ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'")))) {
      v = v.slice(1, -1);
    }
    meta[m[1]] = v;
  }
  return meta;
}

async function main() {
  await fs.mkdir(outDir, { recursive: true });
  const entries = (await fs.readdir(contentDir)).filter((f) => f.endsWith(".md")).sort();
  const manifest = {};
  let built = 0;
  const problems = [];

  for (const file of entries) {
    const slug = file.replace(/\.md$/, "");
    const meta = parseFrontMatter(await fs.readFile(path.join(contentDir, file), "utf8"));
    if (!meta.source) {
      problems.push(`${slug}: missing source`);
      continue;
    }
    if (!meta.permission) {
      problems.push(`${slug}: refused, no permission recorded (FSD D-15)`);
      continue;
    }
    if (!meta.alt) {
      problems.push(`${slug}: missing alt text (NFR-A11Y-02)`);
      continue;
    }
    const src = meta.source.startsWith("public/")
      ? path.join(webRoot, meta.source)
      : path.join(sourceDir, meta.source);
    const sizes = {};
    let info;
    try {
      info = await sharp(src, { failOn: "none" }).metadata();
      for (const w of WIDTHS) {
        const out = path.join(outDir, `${slug}-${w}.webp`);
        const result = await sharp(src, { failOn: "none" })
          .rotate()
          .resize({ width: w, withoutEnlargement: true })
          .webp({ quality: 82, effort: 5 })
          .toFile(out); // no withMetadata(): EXIF, ICC and GPS are dropped
        sizes[w] = { width: result.width, height: result.height, bytes: result.size };
      }
    } catch (e) {
      problems.push(`${slug}: ${e.message}`);
      continue;
    }
    manifest[slug] = {
      width: sizes[1600].width,
      height: sizes[1600].height,
      sizes,
      originalWidth: info.width ?? null,
      originalHeight: info.height ?? null,
    };
    built++;
    console.log(`built ${slug}: ${sizes[1600].width}x${sizes[1600].height} (${Math.round(sizes[1600].bytes / 1024)} KB)`);
  }

  await fs.writeFile(
    path.join(contentDir, "manifest.json"),
    JSON.stringify(manifest, null, 2) + "\n",
    "utf8",
  );
  console.log(`done: ${built} photos, ${problems.length} skipped`);
  for (const p of problems) console.error(`  skip ${p}`);
  if (built === 0 && entries.length > 0) process.exit(1);
}

await main();

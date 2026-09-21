import fs from "node:fs/promises";
import path from "node:path";

// Explicit allow-list of markdown files under
// apps/web/content/articles/ that get exposed at /articles. The file
// staging is deliberate: the repo's `docs/personal/` mixes public
// pieces with drafts / résumés / interview prep, so we don't publish
// straight from there. Files listed here are copies committed to
// this app.
//
// A future "admin content management UI" (backlog item #11) replaces
// this file-list with a database-backed CMS; keep the shape simple
// until then.
const PUBLIC_ARTICLES: readonly ArticleSpec[] = [
  {
    slug: "better-business-decisions-part-1",
    file: "better-business-decisions-part-1.md",
    order: 10,
  },
  {
    slug: "better-business-decisions-part-2",
    file: "better-business-decisions-part-2.md",
    order: 20,
  },
  {
    slug: "better-business-decisions-part-4",
    file: "better-business-decisions-part-4.md",
    order: 40,
  },
  {
    slug: "better-business-decisions-part-5",
    file: "better-business-decisions-part-5.md",
    order: 50,
  },
  {
    slug: "manufacturing-hub-teaser-post",
    file: "manufacturing-hub-teaser-post.md",
    order: 100,
  },
];

type ArticleSpec = {
  slug: string; // URL segment
  file: string; // filename inside apps/web/content/articles/
  order: number; // stable sort key
};

export type ArticleSummary = {
  slug: string;
  title: string;
  subtitle?: string;
  order: number;
};

export type Article = ArticleSummary & {
  body: string; // stripped-of-front-matter markdown
};

// Points at apps/web/content/articles/. process.cwd() is the app
// root in `next dev` and inside the Next.js standalone runtime, the
// `outputFileTracingIncludes` block in next.config.ts pulls the
// content dir into the standalone bundle so runtime reads work.
function contentRoot(): string {
  return path.resolve(process.cwd(), "content/articles");
}

// Parses YAML front matter of the form
//   ---
//   title: "..."
//   subtitle: "..."
//   ---
// Minimal parser, only key: value (with optional quotes) at the top,
// terminated by a `---` line. Anything more complex than that is out
// of scope; front matter is authored by Roger, not user input.
function parseFrontMatter(raw: string): { meta: Record<string, string>; body: string } {
  if (!raw.startsWith("---\n")) return { meta: {}, body: raw };
  const end = raw.indexOf("\n---\n", 4);
  if (end < 0) return { meta: {}, body: raw };
  const yaml = raw.slice(4, end);
  const body = raw.slice(end + 5);
  const meta: Record<string, string> = {};
  for (const line of yaml.split("\n")) {
    const m = line.match(/^([A-Za-z_][A-Za-z0-9_-]*):\s*(.*)$/);
    if (!m) continue;
    let v = m[2].trim();
    if ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'"))) {
      v = v.slice(1, -1);
    }
    meta[m[1]] = v;
  }
  return { meta, body };
}

// Falls back to the first `# ...` heading in body when front matter
// has no title. That covers files like manufacturing-hub-teaser-post.md.
function titleFromBody(body: string): { title: string; body: string } {
  const m = body.match(/^\s*#\s+(.+?)\s*\n/);
  if (!m) return { title: "(untitled)", body };
  return { title: m[1], body: body.slice(m.index! + m[0].length) };
}

async function readOne(spec: ArticleSpec): Promise<Article> {
  const raw = await fs.readFile(path.join(contentRoot(), spec.file), "utf8");
  const { meta, body: afterMeta } = parseFrontMatter(raw);
  let title = meta.title;
  let body = afterMeta;
  if (!title) {
    const t = titleFromBody(afterMeta);
    title = t.title;
    body = t.body;
  }
  return {
    slug: spec.slug,
    title,
    subtitle: meta.subtitle || undefined,
    order: spec.order,
    body,
  };
}

// listArticles returns every publishable article's summary, in
// declared order. Silently skips a spec whose file has gone missing
// so a rename in docs/personal/ doesn't 500 the whole /articles
// page, the caller sees whatever survives.
export async function listArticles(): Promise<ArticleSummary[]> {
  const summaries: ArticleSummary[] = [];
  for (const spec of PUBLIC_ARTICLES) {
    try {
      const a = await readOne(spec);
      summaries.push({
        slug: a.slug,
        title: a.title,
        subtitle: a.subtitle,
        order: a.order,
      });
    } catch {
      /* file missing / unreadable, skip */
    }
  }
  summaries.sort((a, b) => a.order - b.order);
  return summaries;
}

// getArticle returns one article by slug. Returns null when the slug
// isn't in the allow-list OR the file is missing on disk. Rendering
// as 404 is the caller's job.
export async function getArticle(slug: string): Promise<Article | null> {
  const spec = PUBLIC_ARTICLES.find((s) => s.slug === slug);
  if (!spec) return null;
  try {
    return await readOne(spec);
  } catch {
    return null;
  }
}

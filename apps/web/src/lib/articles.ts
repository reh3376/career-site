import fs from "node:fs/promises";
import path from "node:path";

import { parseFrontMatter } from "./frontmatter";

// Explicit allow-list of markdown files under
// apps/web/content/articles/ that get exposed at /articles. The file
// staging is deliberate: the repo's `docs/personal/` mixes public
// pieces with drafts / résumés / interview prep, so we don't publish
// straight from there. Files listed here are copies committed to
// this app.
//
// A future "admin content management UI" (docs/backlog.md, section 4
// "Admin console", the Content management UI entry) replaces
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
    slug: "better-business-decisions-part-3",
    file: "better-business-decisions-part-3.md",
    order: 30,
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
  // Authored in the file's front matter and stable across a rename,
  // which is what separates it from `slug`: the slug is the URL and is
  // derived (from PUBLIC_ARTICLES here, from the filename for photos),
  // while the id names the item itself. FR-CNT-17 lists both for that
  // reason. Validated by scripts/check_content.py, which also enforces
  // that no two content files share one.
  id: string;
  slug: string;
  title: string;
  subtitle?: string;
  // Raw `date:` value from the article's YAML front matter — either a
  // year ("2026") or an ISO-like string ("2026-09-20"). Undefined
  // when the article has no date. The renderer decides how to
  // format it.
  date?: string;
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
    id: meta.id || spec.slug,
    slug: spec.slug,
    title,
    subtitle: meta.subtitle || undefined,
    date: meta.date || undefined,
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
        id: a.id,
        slug: a.slug,
        title: a.title,
        subtitle: a.subtitle,
        date: a.date,
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

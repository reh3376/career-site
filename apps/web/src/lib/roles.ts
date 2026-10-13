import fs from "node:fs/promises";
import path from "node:path";

import { parseFrontMatter } from "./frontmatter";

// The career timeline (FR-CNT-01).
//
// Read from apps/web/content/roles/*.md rather than an allow-list,
// unlike articles.ts: every file in that directory is a role and there
// is no staging problem to solve here, because the directory was
// created for this and holds nothing else. check_content.py validates
// the front matter of all of them in CI, so a file that would render
// half-blank fails the build instead.
//
// The authority for every date, title and organisation in those files
// is docs/personal/career-facts.md, which is also what the JD reviewer
// is given. Nothing here should be edited from memory: if a date is
// wrong it is wrong in the facts sheet first, and both move together.

export type Role = {
  id: string;
  title: string;
  organization: string;
  location: string;
  start: number;
  end: number;
  technologies: string[];
  body: string;
};

const DIR = path.join(process.cwd(), "content", "roles");

function toInt(v: string | undefined): number {
  const n = Number.parseInt((v ?? "").trim(), 10);
  return Number.isFinite(n) ? n : 0;
}

export async function listRoles(): Promise<Role[]> {
  let names: string[];
  try {
    names = await fs.readdir(DIR);
  } catch {
    // A missing directory is an empty timeline, not a crashed page.
    return [];
  }

  const rows: { order: number; role: Role }[] = [];
  for (const name of names.filter((n) => n.endsWith(".md"))) {
    const raw = await fs.readFile(path.join(DIR, name), "utf8");
    const { meta, body } = parseFrontMatter(raw);

    // Public only. `visibility` is in the schema for the day a role is
    // written up for members before it is published, and reading it
    // here is what makes that possible without a second mechanism.
    if ((meta.visibility ?? "").trim() !== "public") continue;

    rows.push({
      order: toInt(meta.order),
      role: {
        id: (meta.id ?? name).trim(),
        title: (meta.title ?? "").trim(),
        organization: (meta.organization ?? "").trim(),
        location: (meta.location ?? "").trim(),
        start: toInt(meta.start),
        end: toInt(meta.end),
        technologies: (meta.technologies ?? "")
          .split(",")
          .map((t) => t.trim())
          .filter(Boolean),
        body: body.trim(),
      },
    });
  }

  // Most recent first, by `order` from the file rather than by date.
  // Two roles can share a year, and the file says which came first;
  // sorting on the dates alone would leave that order to chance.
  rows.sort((a, b) => a.order - b.order);
  return rows.map((r) => r.role);
}

// The span the timeline covers, for the page's own summary line.
// Derived rather than written down, so it cannot disagree with the
// roles underneath it.
export function careerSpan(roles: Role[]): { from: number; to: number } {
  const starts = roles.map((r) => r.start).filter(Boolean);
  const ends = roles.map((r) => r.end).filter(Boolean);
  return {
    from: starts.length ? Math.min(...starts) : 0,
    to: ends.length ? Math.max(...ends) : 0,
  };
}

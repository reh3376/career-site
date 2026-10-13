import fs from "node:fs/promises";
import path from "node:path";

import { parseFrontMatter } from "./frontmatter";

// The project catalog (FR-CNT-03) and its detail pages (FR-CNT-05).
//
// Same shape as roles.ts, read from content/projects/*.md and
// validated by check_content.py in CI. The two are deliberately alike:
// a reader moving between the timeline and the catalog should not be
// able to tell they were built a week apart.
//
// A project is not a repository. Most of these have one and link to
// it, but `repo` is optional because a piece of work that never became
// a public repo must still be publishable, and the day that is untrue
// the catalog starts describing GitHub rather than the work.

export type ProjectStatus = "active" | "maintained" | "paused" | "complete";

export type Project = {
  id: string;
  slug: string;
  title: string;
  summary: string;
  role: string;
  status: ProjectStatus;
  technologies: string[];
  repo: string;
  started: number;
  ended: number;
  body: string;
};

const DIR = path.join(process.cwd(), "content", "projects");

function toInt(v: string | undefined): number {
  const n = Number.parseInt((v ?? "").trim(), 10);
  return Number.isFinite(n) ? n : 0;
}

const STATUSES = new Set<ProjectStatus>([
  "active",
  "maintained",
  "paused",
  "complete",
]);

export async function listProjects(): Promise<Project[]> {
  let names: string[];
  try {
    names = await fs.readdir(DIR);
  } catch {
    return [];
  }

  const rows: { order: number; project: Project }[] = [];
  for (const name of names.filter((n) => n.endsWith(".md"))) {
    const raw = await fs.readFile(path.join(DIR, name), "utf8");
    const { meta, body } = parseFrontMatter(raw);
    if ((meta.visibility ?? "").trim() !== "public") continue;

    const status = (meta.status ?? "").trim() as ProjectStatus;

    rows.push({
      order: toInt(meta.order),
      project: {
        id: (meta.id ?? name).trim(),
        slug: (meta.slug ?? "").trim(),
        title: (meta.title ?? "").trim(),
        summary: (meta.summary ?? "").trim(),
        role: (meta.role ?? "").trim(),
        // An unrecognised status falls back to the least confident
        // reading rather than rendering a raw string: the schema
        // already rejects one in CI, and a catalog that invents a
        // state is worse than one that under-claims.
        status: STATUSES.has(status) ? status : "paused",
        technologies: (meta.technologies ?? "")
          .split(",")
          .map((t) => t.trim())
          .filter(Boolean),
        repo: (meta.repo ?? "").trim(),
        started: toInt(meta.started),
        ended: toInt(meta.ended),
        body: body.trim(),
      },
    });
  }

  rows.sort((a, b) => a.order - b.order);
  return rows.map((r) => r.project);
}

export async function getProject(slug: string): Promise<Project | null> {
  const all = await listProjects();
  return all.find((p) => p.slug === slug) ?? null;
}

// How a status reads to somebody who does not work here. "maintained"
// and "paused" are honest words that a reader can misread as the same
// thing, so each one says what it means about the work rather than
// about the repository.
export const STATUS_LABEL: Record<ProjectStatus, string> = {
  active: "In active development",
  maintained: "Maintained, not actively extended",
  paused: "Paused",
  complete: "Complete",
};

import fs from "node:fs/promises";
import path from "node:path";

import { listArticles } from "./articles";
import { parseFrontMatter } from "./frontmatter";
import { listProjects } from "./projects";
import { listRoles } from "./roles";

// The skills matrix (FR-CNT-09).
//
// The row's condition is the whole point of it: *every entry links to
// at least one piece of evidence*. A skills matrix without that is a
// table of self-assessments, which is the thing a reader already
// discounts on a CV, and putting it on a web page does not make it
// more checkable.
//
// So evidence is not free text. A skill names refs in the form
// `kind:id`, and `resolveEvidence` looks each one up in the content
// that actually exists. A ref that resolves to nothing is dropped and
// reported, never rendered as a dead link, and `skills.test.ts` fails
// on any skill left with no evidence at all. That is what keeps the
// requirement true as the content underneath it changes: rename a
// project slug and the test tells you which skill just lost its
// support.

export type SkillDepth = "working" | "deep" | "owned";

export type Evidence = { label: string; href: string };

export type Skill = {
  id: string;
  title: string;
  category: string;
  depth: SkillDepth;
  years: number;
  evidence: Evidence[];
  /** Refs that named nothing. Empty in a healthy tree. */
  unresolved: string[];
  body: string;
};

const DIR = path.join(process.cwd(), "content", "skills");

const DEPTHS = new Set<SkillDepth>(["working", "deep", "owned"]);

// What each level claims, in words a reader can weigh. Three levels
// rather than five: the difference between a self-scored 7 and 8 is
// not something anybody can evidence.
export const DEPTH_LABEL: Record<SkillDepth, string> = {
  working: "Working knowledge",
  deep: "Deep, hands-on",
  owned: "Owned it, end to end",
};

function toInt(v: string | undefined): number {
  const n = Number.parseInt((v ?? "").trim(), 10);
  return Number.isFinite(n) ? n : 0;
}

/**
 * Turns `kind:id` refs into links, dropping any that name nothing.
 *
 * Exported so the test can assert against the same resolution the page
 * uses, rather than a copy of it that could drift.
 */
export async function resolveEvidence(
  refs: string[],
): Promise<{ evidence: Evidence[]; unresolved: string[] }> {
  const [roles, projects, articles] = await Promise.all([
    listRoles(),
    listProjects(),
    listArticles(),
  ]);

  const evidence: Evidence[] = [];
  const unresolved: string[] = [];

  for (const raw of refs) {
    const [kind, ...rest] = raw.split(":");
    const id = rest.join(":").trim();
    switch (kind.trim()) {
      case "role": {
        const r = roles.find((x) => x.id === id);
        if (r) {
          // The timeline is one page, so a role is an anchor on it
          // rather than a route of its own.
          evidence.push({
            label: `${r.title}, ${r.organization}`,
            href: `/timeline#${r.id}`,
          });
        } else unresolved.push(raw);
        break;
      }
      case "project": {
        const p = projects.find((x) => x.slug === id);
        if (p) evidence.push({ label: p.title, href: `/projects/${p.slug}` });
        else unresolved.push(raw);
        break;
      }
      case "article": {
        const a = articles.find((x) => x.slug === id);
        if (a) evidence.push({ label: a.title, href: `/articles/${a.slug}` });
        else unresolved.push(raw);
        break;
      }
      default:
        unresolved.push(raw);
    }
  }
  return { evidence, unresolved };
}

export async function listSkills(): Promise<Skill[]> {
  let names: string[];
  try {
    names = await fs.readdir(DIR);
  } catch {
    return [];
  }

  const rows: { order: number; skill: Skill }[] = [];
  for (const name of names.filter((n) => n.endsWith(".md"))) {
    const raw = await fs.readFile(path.join(DIR, name), "utf8");
    const { meta, body } = parseFrontMatter(raw);
    if ((meta.visibility ?? "").trim() !== "public") continue;

    const depth = (meta.depth ?? "").trim() as SkillDepth;
    const refs = (meta.evidence ?? "")
      .split(",")
      .map((r) => r.trim())
      .filter(Boolean);
    const { evidence, unresolved } = await resolveEvidence(refs);

    // A skill whose evidence all failed to resolve is not rendered.
    // The row says every entry links to evidence; showing one that
    // does not would make the page a liar about its own rule, and the
    // test turns this into a build failure rather than a quiet absence.
    if (evidence.length === 0) continue;

    rows.push({
      order: toInt(meta.order),
      skill: {
        id: (meta.id ?? name).trim(),
        title: (meta.title ?? "").trim(),
        category: (meta.category ?? "").trim(),
        depth: DEPTHS.has(depth) ? depth : "working",
        years: toInt(meta.years),
        evidence,
        unresolved,
        body: body.trim(),
      },
    });
  }

  rows.sort((a, b) => a.order - b.order);
  return rows.map((r) => r.skill);
}

/** Grouped for rendering, in first-appearance order rather than alphabetical. */
export function byCategory(skills: Skill[]): { category: string; skills: Skill[] }[] {
  const out: { category: string; skills: Skill[] }[] = [];
  for (const s of skills) {
    const found = out.find((g) => g.category === s.category);
    if (found) found.skills.push(s);
    else out.push({ category: s.category, skills: [s] });
  }
  return out;
}

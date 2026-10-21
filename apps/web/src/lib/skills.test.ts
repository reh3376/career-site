import { readdirSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import { listArticles } from "./articles";
import { listProjects } from "./projects";
import { listRoles } from "./roles";
import { byCategory, DEPTH_LABEL, listSkills, resolveEvidence } from "./skills";

// FR-CNT-09's condition is that *every* entry links to at least one
// piece of evidence. That is a promise the content can break on its
// own, without anyone touching this code: rename a project slug,
// retitle a role id, and a skill silently loses its support.
//
// So these tests resolve the refs against the published content rather
// than against a fixture. If a rename ever orphans a skill, the failure
// names which one, which is the whole reason the matrix is worth
// publishing.

describe("the skills matrix", () => {
  it("gives every published skill at least one resolved piece of evidence", async () => {
    const skills = await listSkills();
    expect(skills.length).toBeGreaterThan(0);

    for (const s of skills) {
      expect(
        s.evidence.length,
        `${s.id} has no evidence, which is the one thing FR-CNT-09 requires`,
      ).toBeGreaterThan(0);
    }
  });

  // The loader drops a skill whose refs all fail, so the test above
  // can never fail on its own: a broken ref makes a skill disappear
  // rather than appear unsupported. This is the test that notices.
  it("publishes every skill file, so none was dropped for unresolvable evidence", async () => {
    // Counted from disk rather than hardcoded. The literal that used to
    // be here said 15, which meant adding a skill failed this test for
    // the wrong reason and taught whoever hit it to bump the number.
    // Worse, it would have passed while hiding a drop: add one file and
    // orphan another on the same day and the count still matches.
    const files = readdirSync(join(process.cwd(), "content", "skills")).filter(
      (f) => f.endsWith(".md"),
    );
    const skills = await listSkills();
    expect(
      skills.length,
      "a skill file exists that resolved to nothing and was dropped from the page",
    ).toBe(files.length);
  });

  it("resolves every single ref, not merely one per skill", async () => {
    const skills = await listSkills();
    const orphaned = skills.flatMap((s) =>
      s.unresolved.map((u) => `${s.id} -> ${u}`),
    );
    expect(orphaned, "evidence refs that name nothing").toEqual([]);
  });

  it("points every piece of evidence at content that exists", async () => {
    const [roles, projects, articles] = await Promise.all([
      listRoles(),
      listProjects(),
      listArticles(),
    ]);
    const known = new Set([
      ...roles.map((r) => `/timeline#${r.id}`),
      ...projects.map((p) => `/projects/${p.slug}`),
      ...articles.map((a) => `/articles/${a.slug}`),
    ]);

    for (const s of await listSkills()) {
      for (const e of s.evidence) {
        expect(known.has(e.href), `${s.id} links to ${e.href}`).toBe(true);
      }
    }
  });

  it("reports an unknown ref rather than inventing a link for it", async () => {
    const { evidence, unresolved } = await resolveEvidence([
      "project:no-such-project",
      "role:no-such-role",
      "nonsense",
    ]);
    expect(evidence).toEqual([]);
    expect(unresolved.length).toBe(3);
  });

  it("gives every skill a depth label and a plausible span", async () => {
    for (const s of await listSkills()) {
      expect(DEPTH_LABEL[s.depth], `depth label for ${s.id}`).toBeTruthy();
      expect(s.years, `years for ${s.id}`).toBeGreaterThan(0);
      // Nothing can predate the first role in the timeline.
      expect(s.years, `years for ${s.id}`).toBeLessThanOrEqual(30);
    }
  });

  it("groups by category in first-appearance order", async () => {
    const groups = byCategory(await listSkills());
    expect(groups.length).toBeGreaterThan(1);
    expect(groups.map((g) => g.category)).toEqual([
      ...new Set(groups.map((g) => g.category)),
    ]);
  });
});

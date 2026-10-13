import { describe, expect, it } from "vitest";

import { getProject, listProjects, STATUS_LABEL } from "./projects";

// Read off the committed files, for the same reason roles.test.ts does:
// the failure most likely to happen here is content, not parsing. A
// slug that stops matching its filename, a status typo, two projects
// with the same slug so one detail page shadows the other. A fixture
// would pass through every one of those.

describe("the project catalog", () => {
  it("publishes projects, each with what the catalog row renders", async () => {
    const projects = await listProjects();
    expect(projects.length).toBeGreaterThan(0);

    for (const p of projects) {
      expect(p.id, "id").toBeTruthy();
      expect(p.slug, `slug for ${p.id}`).toBeTruthy();
      expect(p.title, `title for ${p.id}`).toBeTruthy();
      expect(p.summary, `summary for ${p.id}`).toBeTruthy();
      expect(p.role, `role for ${p.id}`).toBeTruthy();
      expect(p.body, `body for ${p.id}`).toBeTruthy();
      expect(p.technologies.length, `technologies for ${p.id}`).toBeGreaterThan(0);
      // Every status must have a label, or the catalog renders a blank
      // column where the state should be.
      expect(STATUS_LABEL[p.status], `label for status ${p.status}`).toBeTruthy();
    }
  });

  it("gives each project a distinct slug, since the detail route keys on it", async () => {
    const projects = await listProjects();
    const slugs = projects.map((p) => p.slug);
    expect(new Set(slugs).size).toBe(slugs.length);
  });

  it("resolves every catalog entry to a detail page", async () => {
    const projects = await listProjects();
    for (const p of projects) {
      const found = await getProject(p.slug);
      expect(found?.id, `/projects/${p.slug} should resolve`).toBe(p.id);
    }
  });

  it("returns null for an unknown slug rather than throwing", async () => {
    expect(await getProject("no-such-project")).toBeNull();
  });

  // FR-CNT-05 asks a detail page to say what the problem was and what
  // was done about it. A project whose body is only a summary restated
  // is a catalog entry with a bigger font.
  it("gives every detail page more than a restated summary", async () => {
    const projects = await listProjects();
    for (const p of projects) {
      expect(
        p.body.length,
        `${p.slug} has a body of ${p.body.length} characters`,
      ).toBeGreaterThan(p.summary.length * 2);
      expect(p.body, `${p.slug} should say what the problem was`).toMatch(
        /##\s/,
      );
    }
  });
});

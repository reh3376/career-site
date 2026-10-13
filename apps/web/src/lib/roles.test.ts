import { describe, expect, it } from "vitest";

import { careerSpan, listRoles, type Role } from "./roles";

// The timeline is read off the committed role files rather than a
// fixture, because the thing most likely to break here is the content,
// not the parser: a date edited in the wrong direction, a role whose
// `order` collides with another, a `visibility` typo that silently
// removes a role from the page. A fixture would pass while the real
// page lost an entry.
//
// Every date these assert against comes from docs/personal/career-facts.md.

describe("the career timeline", () => {
  it("publishes every role, newest first, with no gaps in the sequence", async () => {
    const roles = await listRoles();

    expect(roles.length).toBe(7);

    // Descending by start year. Written as a comparison between
    // neighbours rather than against a hard-coded list, so adding a
    // role keeps the test meaningful instead of just failing a count.
    for (let i = 1; i < roles.length; i += 1) {
      expect(
        roles[i - 1].start,
        `${roles[i - 1].organization} should start after ${roles[i].organization}`,
      ).toBeGreaterThanOrEqual(roles[i].start);
    }

    // The two ends, which are the figures the landing page's "thirty
    // years" claim rests on.
    expect(roles[0].organization).toBe("Whiskey House");
    expect(roles[0].end).toBe(2026);
    expect(roles[roles.length - 1].organization).toBe(
      "Columbia Gas Transmission",
    );
    expect(roles[roles.length - 1].start).toBe(1996);
  });

  it("gives every role the fields the page renders, so none renders half-blank", async () => {
    const roles = await listRoles();
    for (const r of roles) {
      expect(r.id, "id").toBeTruthy();
      expect(r.title, `title for ${r.id}`).toBeTruthy();
      expect(r.organization, `organization for ${r.id}`).toBeTruthy();
      expect(r.location, `location for ${r.id}`).toBeTruthy();
      expect(r.body, `body for ${r.id}`).toBeTruthy();
      expect(r.technologies.length, `technologies for ${r.id}`).toBeGreaterThan(0);
      // A role that has not ended renders as ending in year zero, which
      // is worse than failing here.
      expect(r.end, `end for ${r.id}`).toBeGreaterThan(r.start);
    }
  });

  it("gives each role a distinct id, since the page keys on it", async () => {
    const roles = await listRoles();
    expect(new Set(roles.map((r) => r.id)).size).toBe(roles.length);
  });

  it("derives the span from the roles rather than from a written-down number", () => {
    const roles = [
      { start: 2017, end: 2022 },
      { start: 1996, end: 1998 },
      { start: 2022, end: 2026 },
    ] as Role[];
    expect(careerSpan(roles)).toEqual({ from: 1996, to: 2026 });
  });

  it("reports an empty span rather than throwing when there are no roles", () => {
    expect(careerSpan([])).toEqual({ from: 0, to: 0 });
  });
});

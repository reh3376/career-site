import { describe, expect, it } from "vitest";

import {
  inSection,
  listCredentials,
  SECTIONS,
  STATUS_LABEL,
} from "./credentials";

// Read off the committed files, like roles and projects, because the
// failure that matters here is a content one: a category typo that
// drops an entry into no section at all, or a lapsed certification
// quietly losing its `lapsed` and being read as current.

describe("awards and credentials", () => {
  it("publishes every file, with none lost to an unrecognised category", async () => {
    const credentials = await listCredentials();
    expect(credentials.length).toBe(12);
  });

  it("renders every entry in exactly one section", async () => {
    const credentials = await listCredentials();
    const placed = SECTIONS.flatMap(({ category }) =>
      inSection(credentials, category),
    );
    // Same length means every credential landed in a section and none
    // landed in two. A category the page has no section for would make
    // an entry vanish from the page while still existing in the tree.
    expect(placed.length).toBe(credentials.length);
    expect(new Set(placed.map((c) => c.id)).size).toBe(credentials.length);
  });

  it("gives every entry the fields its row renders", async () => {
    for (const c of await listCredentials()) {
      expect(c.id, "id").toBeTruthy();
      expect(c.title, `title for ${c.id}`).toBeTruthy();
      expect(c.category, `category for ${c.id}`).toBeTruthy();
    }
  });

  // The point of publishing a lapsed certification is the word
  // "lapsed". A certification that loses its status reads as current,
  // which is a claim rather than an omission.
  it("keeps every certification's status, so none can read as current", async () => {
    const certs = inSection(await listCredentials(), "certification");
    expect(certs.length).toBeGreaterThan(0);
    for (const c of certs) {
      expect(c.status, `status for ${c.id}`).toBeTruthy();
      if (c.status === "lapsed") {
        expect(STATUS_LABEL[c.status], "a lapsed cert must render a label").toBe(
          "lapsed",
        );
      }
    }
  });

  it("has an award and a degree, since the row names both", async () => {
    const credentials = await listCredentials();
    expect(inSection(credentials, "award").length).toBeGreaterThan(0);
    expect(inSection(credentials, "education").length).toBeGreaterThan(0);
    expect(inSection(credentials, "service").length).toBeGreaterThan(0);
  });
});

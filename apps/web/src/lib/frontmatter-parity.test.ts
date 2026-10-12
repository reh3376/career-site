import fs from "node:fs";
import path from "node:path";

import { describe, expect, it } from "vitest";

import { parseFrontMatter } from "./frontmatter";

// One fixture, three parsers. See testdata/frontmatter/README.md.
//
// The Go half of this is
// services/api/internal/ingest/frontmatter_parity_test.go and reads the
// same two files, so a change to one parser's behaviour fails on the
// other side of the repository rather than being discovered by a reader
// noticing a field stopped working.

const FIXTURE_DIR = path.join(process.cwd(), "..", "..", "testdata", "frontmatter");

function readFixture() {
  const raw = fs.readFileSync(path.join(FIXTURE_DIR, "fixture.md"), "utf8");
  const expected = JSON.parse(
    fs.readFileSync(path.join(FIXTURE_DIR, "expected.json"), "utf8"),
  ) as Record<string, string>;
  return { raw, expected };
}

describe("front matter parity", () => {
  it("parses the shared fixture to exactly the shared expectation", () => {
    const { raw, expected } = readFixture();
    // A fixture that failed to load would make every assertion below
    // vacuous, which is the way this kind of test usually rots.
    expect(Object.keys(expected).length).toBeGreaterThan(5);

    const { meta } = parseFrontMatter(raw);
    expect(meta).toEqual(expected);
  });

  it("returns the body with the front matter stripped", () => {
    const { raw } = readFixture();
    const { body } = parseFrontMatter(raw);
    expect(body.startsWith("\nBody text")).toBe(true);
  });

  // Its own case rather than part of the fixture: a file with mixed
  // line endings does not survive a checkout reliably. articles.ts did
  // not normalise CRLF until the parsers were consolidated, so a file
  // saved with Windows line endings parsed to an empty map there and a
  // full one in the Go walker.
  it("normalises CRLF, as the Go parser does", () => {
    const { meta } = parseFrontMatter(
      '---\r\ntitle: "carriage returns"\r\n---\r\nbody\r\n',
    );
    expect(meta.title).toBe("carriage returns");
  });

  // Without the length guard, a value that is a single quote character
  // slices to the empty string.
  it("leaves a lone quote character alone", () => {
    const { meta } = parseFrontMatter('---\nlone: "\n---\nbody\n');
    expect(meta.lone).toBe('"');
  });
});

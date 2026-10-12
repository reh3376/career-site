// The one front-matter parser on the TypeScript side.
//
// There were two, in lib/articles.ts and lib/photos.ts, near-identical
// and independently maintained, alongside a third in Go
// (services/api/internal/ingest/walker.go) whose comment said it handled
// "the same shape the TypeScript articles library expects (kept in sync
// by convention)".
//
// They had drifted, in two ways that were found only by comparing them
// on 2026-10-12:
//
//   * articles.ts did not normalise CRLF. photos.ts and the Go walker
//     did. A file saved with Windows line endings parsed to an empty
//     map there and to a full one everywhere else.
//   * articles.ts stripped the first and last character of any value
//     starting and ending with a quote, without checking the length, so
//     a value that was a single `"` became an empty string. The other
//     two left it alone.
//
// Neither mattered for the content committed today, which is exactly
// why neither was noticed. Both are now pinned by a fixture all three
// parsers are checked against: testdata/frontmatter/.
//
// This deliberately does not pull in a YAML library. The shape is what
// we author, the Go side has no equivalent dependency, and adding one
// on this side only would reintroduce the divergence this file exists
// to end.

export type FrontMatter = {
  meta: Record<string, string>;
  body: string;
};

// Identifier-ish keys only, matching frontMatterKV in walker.go.
const KV = /^([A-Za-z_][A-Za-z0-9_-]*):\s*(.*)$/;

export function parseFrontMatter(raw: string): FrontMatter {
  const text = raw.replace(/\r\n/g, "\n");
  if (!text.startsWith("---\n")) return { meta: {}, body: raw };
  const end = text.indexOf("\n---\n", 4);
  if (end < 0) return { meta: {}, body: raw };

  const meta: Record<string, string> = {};
  for (const line of text.slice(4, end).split("\n")) {
    const m = KV.exec(line);
    if (!m) continue;
    let v = m[2].trim();
    // The length guard matters: without it a value that is a single
    // quote character slices to the empty string.
    if (
      v.length >= 2 &&
      ((v.startsWith('"') && v.endsWith('"')) ||
        (v.startsWith("'") && v.endsWith("'")))
    ) {
      v = v.slice(1, -1);
    }
    meta[m[1]] = v;
  }
  return { meta, body: text.slice(end + 5) };
}

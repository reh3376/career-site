# One fixture, four parsers

This repository parses YAML front matter four times, in three
languages, and none of them uses a YAML library:

| Parser | Used for |
|---|---|
| `apps/web/src/lib/articles.ts` | the published articles |
| `apps/web/src/lib/photos.ts` | the photo gallery |
| `services/api/internal/ingest/walker.go` | corpus ingestion |
| `scripts/check_content.py` | the build-time schema check |

The Go one's own comment said it "handles the shape we author, which is
the same shape the TypeScript articles library expects (kept in sync by
convention)". Convention is not a mechanism, which is the whole subject
of ADR 0031, and the two had in fact drifted by the time anyone checked
on 2026-10-12: `articles.ts` did not normalise CRLF, and it stripped a
lone `"` to an empty string where the other two left it alone.

`fixture.md` and `expected.json` are what they are now checked against.
Every parser must produce exactly `expected.json` from `fixture.md`. A
parser that gains a feature the others lack fails here first.

The cases are chosen for the ways these four could plausibly disagree,
not for YAML coverage: quoting styles, a value containing the delimiter,
a lone quote character, trailing whitespace outside the quotes, a list
key (whose value is the empty string, because none of these parsers
reads list items), and an empty value.

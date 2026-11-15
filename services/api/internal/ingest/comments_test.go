package ingest

import (
	"strings"
	"testing"

	"context"
	"github.com/reh3376/career-site/services/api/internal/users"
	"os"
	"path/filepath"
)

// The leak these tests exist to prevent: a scrubbed corpus copy
// carries an HTML comment saying what was removed, and that note was
// chunked, embedded, retrievable, and used as the document title.
func TestStripHTMLComments(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "the real case: multi-line scrubbing note above the heading",
			in: "<!-- Scrubbed copy for corpus ingestion.\n" +
				"Three passages were removed: the ownership structure,\n" +
				"a vendor named in a procurement disagreement, and a\n" +
				"statement that a former employer's customers lost trust. -->\n" +
				"\n# General Interview Prep\n\nBody text.\n",
			want: "# General Interview Prep\n\nBody text.",
		},
		{
			name: "single line comment",
			in:   "<!-- note -->\n# Title\n",
			want: "# Title",
		},
		{
			name: "comment at end of a content line keeps the content",
			in:   "Revenue grew 40%. <!-- source: internal deck -->\n",
			want: "Revenue grew 40%.",
		},
		{
			name: "two comments on one line",
			in:   "a <!-- x --> b <!-- y --> c\n",
			want: "a  b  c",
		},
		{
			name: "no comments is unchanged",
			in:   "# Title\n\nPlain body.",
			want: "# Title\n\nPlain body.",
		},
		{
			name: "comment inside a fenced block survives, because READMEs show examples",
			in:   "# Title\n\n```html\n<!-- this is how you comment -->\n```\n\nAfter.",
			want: "# Title\n\n```html\n<!-- this is how you comment -->\n```\n\nAfter.",
		},
		{
			name: "tilde fences too",
			in:   "~~~\n<!-- kept -->\n~~~",
			want: "~~~\n<!-- kept -->\n~~~",
		},
		{
			name: "a comment outside a fence is stripped even when the doc also has fences",
			in:   "<!-- gone -->\n# T\n\n```\n<!-- kept -->\n```",
			want: "# T\n\n```\n<!-- kept -->\n```",
		},
		{
			name: "unterminated comment strips to the end rather than leaking the tail",
			in:   "# Title\n\n<!-- oops forgot to close\nsecret passage here\nmore secrets",
			want: "# Title",
		},
		{
			name: "a fence marker inside a comment is not a fence",
			in:   "<!--\n```\nstill commented\n```\n-->\nAfter.",
			want: "After.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripHTMLComments(tc.in); got != tc.want {
				t.Errorf("stripHTMLComments()\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

// The title is derived from the body, so a stripped comment must not
// be able to become one. This is the half of the defect that would
// have shown up in a citation label on a member's screen.
func TestTitleIsNotTakenFromAComment(t *testing.T) {
	body := stripHTMLComments("<!-- Scrubbed copy for corpus ingestion. The original,\n" +
		"reh-interview-prep-general.md, is unchanged. -->\n\n" +
		"# General Interview Prep — Roger E. Henley II\n\nBody.")

	got := firstNonEmptyLine(body)
	want := "General Interview Prep — Roger E. Henley II"
	if got != want {
		t.Errorf("firstNonEmptyLine() = %q, want %q", got, want)
	}
	if strings.Contains(got, "<!--") || strings.Contains(got, "Scrubbed") {
		t.Errorf("title still carries the scrubbing note: %q", got)
	}
}

// A document that is nothing but a comment must fail loudly rather
// than ingest as an empty shell.
func TestCommentOnlyDocumentIsEmpty(t *testing.T) {
	if got := stripHTMLComments("<!-- just a note -->"); got != "" {
		t.Errorf("want empty so IngestText returns ErrEmpty, got %q", got)
	}
}

// The regression this pins: the comment fix alone was not enough.
//
// Walk derived the title itself, from front-matter-stripped but
// still-commented text, and passed it to IngestText as a non-empty
// Title. A non-empty Title wins over the body, so after the first fix
// the chunks were clean and the document was still called
// "<!-- Scrubbed copy for corpus ingestion. The original,". The title
// now comes from whatever IngestText sees, which is comment-free.
//
// Asserted against the two functions in the order the walker and
// IngestText call them, because a unit test of either one alone is
// what let the first fix look complete.
func TestDeclaredTitleWinsButDerivedTitleIsCommentFree(t *testing.T) {
	const scrubbed = "<!-- Scrubbed copy for corpus ingestion. The original,\n" +
		"reh-interview-prep-general.md, is unchanged. -->\n\n" +
		"# General Interview Prep — Roger E. Henley II\n\nBody.\n"

	t.Run("no declared title: derived from the stripped body", func(t *testing.T) {
		meta, body := parseFrontMatter(scrubbed)
		if meta["title"] != "" {
			t.Fatalf("expected no declared title, got %q", meta["title"])
		}
		// What IngestText does with it.
		got := firstNonEmptyLine(stripHTMLComments(body))
		want := "General Interview Prep — Roger E. Henley II"
		if got != want {
			t.Errorf("title = %q, want %q", got, want)
		}
	})

	t.Run("declared title still wins", func(t *testing.T) {
		meta, _ := parseFrontMatter("---\ntitle: Declared\n---\n<!-- note -->\n# Derived\n")
		if meta["title"] != "Declared" {
			t.Errorf("declared title = %q, want %q", meta["title"], "Declared")
		}
	})
}

// The reindex short-circuit. It skipped the document whose title this
// package had just learned to compute differently, because the body
// had not moved, so the stale title survived two deploys and two
// reindexes.
func TestUnchangedComparesEveryColumnTheUpsertWrites(t *testing.T) {
	hash := []byte{1, 2, 3}
	meta := []byte(`{"a":1}`)
	base := &users.CorpusDocument{
		Title:       "General Interview Prep",
		Visibility:  users.VisibilityCorpusOnly,
		Meta:        meta,
		ContentHash: hash,
	}

	if !unchanged(base, base.Title, base.Visibility, meta, hash) {
		t.Error("identical document should skip")
	}
	// The real regression: same body, different title.
	if unchanged(base, "<!-- Scrubbed copy for corpus ingestion. The original,", base.Visibility, meta, hash) {
		t.Error("a title change on unchanged text must not skip")
	}
	if unchanged(base, base.Title, users.VisibilityPublic, meta, hash) {
		t.Error("a visibility change on unchanged text must not skip")
	}
	if unchanged(base, base.Title, base.Visibility, []byte(`{"a":2}`), hash) {
		t.Error("a meta change on unchanged text must not skip")
	}
	if unchanged(base, base.Title, base.Visibility, meta, []byte{9, 9, 9}) {
		t.Error("a body change must not skip")
	}
	if unchanged(nil, base.Title, base.Visibility, meta, hash) {
		t.Error("a document that does not exist yet must not skip")
	} // Meta is compared as a value. Postgres returns jsonb in its own
	// spelling and stores an absent meta as an empty object; comparing
	// bytes called every document changed on every reindex.
	stored := &users.CorpusDocument{Title: base.Title, Visibility: base.Visibility, ContentHash: hash,
		Meta: []byte(`{"end": "2022", "start": "2018", "organization": "Acme"}`)}
	if !unchanged(stored, base.Title, base.Visibility, []byte(`{"organization":"Acme","start":"2018","end":"2022"}`), hash) {
		t.Error("the same meta in Postgres's spelling should skip")
	}
	empty := &users.CorpusDocument{Title: base.Title, Visibility: base.Visibility, ContentHash: hash, Meta: []byte(`{}`)}
	if !unchanged(empty, base.Title, base.Visibility, nil, hash) {
		t.Error("no meta sent and an empty object stored are the same")
	}
	if unchanged(empty, base.Title, base.Visibility, []byte(`{"start":"2018"}`), hash) {
		t.Error("meta added to a document that had none must not skip")
	}
}

func TestDocumentMetaKeepsEngagementFrontMatter(t *testing.T) {
	role := map[string]string{"title": "Director", "organization": "Acme", "start": "2018", "end": "2022", "order": "20", "technologies": "x"}
	got := string(documentMeta("role", role))
	if got != `{"end":"2022","order":"20","organization":"Acme","start":"2018"}` {
		t.Errorf("role meta = %s", got)
	}
	project := map[string]string{"title": "P", "started": "2026", "status": "active"}
	if got := string(documentMeta("project", project)); got != `{"start":"2026","status":"active"}` {
		t.Errorf("project meta = %s", got)
	}
	if documentMeta("article", role) != nil {
		t.Error("only roles and projects carry engagement meta")
	}
	if documentMeta("role", map[string]string{"title": "x"}) != nil {
		t.Error("a role that states none of the keys has no meta")
	}
}

type metaStore struct {
	stored map[string]string
	writes int
}

func (m *metaStore) SetCorpusDocumentMeta(_ context.Context, kind, path string, meta []byte) (bool, error) {
	key := kind + "/" + path
	old, exists := m.stored[key]
	// A document that is not in the corpus is not created, and one whose
	// meta already matches is not written.
	if !exists || old == string(meta) {
		return false, nil
	}
	m.stored[key] = string(meta)
	m.writes++
	return true, nil
}

// The boot-time sync writes a role's front matter onto the document
// that already exists for it, leaves an unknown file and an unrelated
// kind alone, and writes nothing the second time.
func TestSyncEngagementMeta(t *testing.T) {
	root := t.TempDir()
	write := func(dir, name, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("roles", "01-acme.md", "---\ntitle: Director\norganization: Acme\nstart: 2018\nend: 2022\n---\nBody.\n")
	write("roles", "02-new.md", "---\ntitle: New\norganization: Beta\nstart: 2023\n---\nBody.\n")
	write("projects", "10-thing.md", "---\ntitle: Thing\nstarted: 2026\nstatus: active\n---\nBody.\n")
	write("articles", "a.md", "---\ntitle: An article\nstart: 2020\n---\nBody.\n")
	store := &metaStore{stored: map[string]string{"role/01-acme.md": "{}", "project/10-thing.md": "{}"}}
	dirs := map[string]string{"role": filepath.Join(root, "roles"), "project": filepath.Join(root, "projects"), "article": filepath.Join(root, "articles")}
	n, err := SyncEngagementMeta(context.Background(), store, dirs)
	if err != nil || n != 2 {
		t.Fatalf("first sync: %d (err %v)", n, err)
	}
	if got := store.stored["role/01-acme.md"]; got != `{"end":"2022","organization":"Acme","start":"2018"}` {
		t.Errorf("the role's meta: %s", got)
	}
	if got := store.stored["project/10-thing.md"]; got != `{"start":"2026","status":"active"}` {
		t.Errorf("the project's meta: %s", got)
	}
	if _, made := store.stored["role/02-new.md"]; made {
		t.Error("a file with no document yet is left for the reindex")
	}
	if n, err := SyncEngagementMeta(context.Background(), store, dirs); err != nil || n != 0 {
		t.Errorf("second sync: %d (err %v), want nothing written", n, err)
	}
}

package ingest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/reh3376/career-site/services/api/internal/users"
)

// WalkOptions configures one directory walk.
type WalkOptions struct {
	Root       string
	SourceKind string
	// Visibility applied to every document under Root. Front matter
	// may tighten this (public → corpus_only) but never loosen it, so
	// a file under the private mount can't opt itself into public.
	Visibility string
}

// WalkResult summarises a directory reindex run.
type WalkResult struct {
	Root           string
	SourceKind     string
	Visibility     string
	FilesScanned   int
	DocsIngested   int // includes skipped-because-unchanged
	DocsSkipped    int // hash matched; no chunks/embeds this run
	ChunksInserted int
	ChunksEmbedded int
	Errors         []string // per-file error strings; not fatal to the run
}

// Add folds another result into r (used to aggregate a multi-kind walk).
func (r *WalkResult) Add(o *WalkResult) {
	r.FilesScanned += o.FilesScanned
	r.DocsIngested += o.DocsIngested
	r.DocsSkipped += o.DocsSkipped
	r.ChunksInserted += o.ChunksInserted
	r.ChunksEmbedded += o.ChunksEmbedded
	r.Errors = append(r.Errors, o.Errors...)
}

// WalkDirectory walks opts.Root for .md / .txt files and ingests each
// one under opts.SourceKind. Uses each file's path relative to Root as
// the source_path, so re-runs land on the same corpus rows.
//
// Errors on individual files (unreadable, empty, ingest failure) are
// collected in the result rather than aborting the walk; one bad file
// shouldn't block the rest.
func (i *Ingester) WalkDirectory(ctx context.Context, opts WalkOptions) (*WalkResult, error) {
	if opts.Root == "" {
		return nil, fmt.Errorf("walker: root is required")
	}
	if opts.SourceKind == "" {
		return nil, fmt.Errorf("walker: source_kind is required")
	}
	if opts.Visibility == "" {
		opts.Visibility = users.VisibilityPublic
	}
	if !users.ValidVisibility(opts.Visibility) {
		return nil, fmt.Errorf("walker: invalid visibility %q", opts.Visibility)
	}
	if _, err := os.Stat(opts.Root); err != nil {
		return nil, fmt.Errorf("walker: root %q not accessible: %w", opts.Root, err)
	}
	res := &WalkResult{Root: opts.Root, SourceKind: opts.SourceKind, Visibility: opts.Visibility}

	err := filepath.WalkDir(opts.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %s", path, err.Error()))
			return nil
		}
		if d.IsDir() || !isTextSource(d.Name()) {
			return nil
		}
		res.FilesScanned++

		body, err := os.ReadFile(path)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: read: %s", path, err.Error()))
			return nil
		}
		meta, stripped := parseFrontMatter(string(body))
		title := meta["title"]
		if title == "" {
			title = firstNonEmptyLine(stripped)
		}
		rel, relErr := filepath.Rel(opts.Root, path)
		if relErr != nil {
			rel = path
		}

		out, err := i.IngestText(ctx, IngestInput{
			SourceKind: opts.SourceKind,
			SourcePath: rel,
			Title:      title,
			Visibility: effectiveVisibility(opts.Visibility, meta["visibility"]),
			Body:       stripped,
		})
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: ingest: %s", rel, err.Error()))
			return nil
		}
		res.DocsIngested++
		if out.Skipped {
			res.DocsSkipped++
		}
		res.ChunksInserted += out.ChunksInserted
		res.ChunksEmbedded += out.ChunksEmbedded
		return nil
	})
	if err != nil {
		return res, fmt.Errorf("walker: %w", err)
	}
	return res, nil
}

// WalkKinds treats each immediate subdirectory of root as a
// source_kind and walks it. Subdirectories whose name is not in
// KnownKinds are reported as errors and skipped, so a mis-named
// folder in the private sync can't create a stray kind. When
// onlyKind is non-empty, just that one subdirectory is walked.
func (i *Ingester) WalkKinds(ctx context.Context, root, visibility, onlyKind string) (*WalkResult, []string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil, fmt.Errorf("walker: root %q not accessible: %w", root, err)
	}
	agg := &WalkResult{Root: root, Visibility: visibility}
	var kinds []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		kind := e.Name()
		if onlyKind != "" && kind != onlyKind {
			continue
		}
		if !IsKnownKind(kind) {
			agg.Errors = append(agg.Errors, fmt.Sprintf("%s: not a known source_kind, skipped", kind))
			continue
		}
		res, err := i.WalkDirectory(ctx, WalkOptions{
			Root:       filepath.Join(root, kind),
			SourceKind: kind,
			Visibility: visibility,
		})
		if err != nil {
			agg.Errors = append(agg.Errors, fmt.Sprintf("%s: %s", kind, err.Error()))
			continue
		}
		agg.Add(res)
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	if onlyKind != "" && len(kinds) == 0 && len(agg.Errors) == 0 {
		return nil, nil, fmt.Errorf("walker: no %q directory under %s", onlyKind, root)
	}
	return agg, kinds, nil
}

// effectiveVisibility lets front matter tighten the walk's visibility
// but never loosen it. A `visibility: public` line in a private file
// is ignored; `visibility: corpus_only` in a public file is honoured.
func effectiveVisibility(base, override string) string {
	if base == users.VisibilityCorpusOnly {
		return base
	}
	if strings.TrimSpace(override) == users.VisibilityCorpusOnly {
		return users.VisibilityCorpusOnly
	}
	return base
}

func isTextSource(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".txt")
}

// parseFrontMatter recognises the minimal YAML front-matter shape
// used by the article corpus:
//
//	---
//	title: "..."
//	subtitle: "..."
//	date: "..."
//	visibility: corpus_only
//	---
//
// Returns the parsed key→value map + the body with front matter
// stripped. Falls through as identity when no front matter is
// present. Deliberately does NOT pull in a YAML parser dependency;
// this handles the shape we author, which is the same shape the
// TypeScript articles library expects (kept in sync by convention).
func parseFrontMatter(raw string) (map[string]string, string) {
	meta := map[string]string{}
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(raw, "---\n") {
		return meta, raw
	}
	end := strings.Index(raw[4:], "\n---\n")
	if end < 0 {
		return meta, raw
	}
	body := raw[4+end+5:]
	for line := range strings.SplitSeq(raw[4:4+end], "\n") {
		m := frontMatterKV.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		v := strings.TrimSpace(m[2])
		if len(v) >= 2 {
			if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
				v = v[1 : len(v)-1]
			}
		}
		meta[m[1]] = v
	}
	return meta, body
}

// frontMatterKV matches `key: value` lines in the front-matter
// block. Restricted to identifier-ish keys.
var frontMatterKV = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_-]*):\s*(.*)$`)

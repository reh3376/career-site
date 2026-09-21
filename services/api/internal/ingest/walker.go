package ingest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// WalkResult summarises a directory reindex run. Fields correspond
// to what /admin/corpus's "reindex" button displays.
type WalkResult struct {
	Root           string
	FilesScanned   int
	DocsIngested   int // includes skipped-because-unchanged
	DocsSkipped    int // hash matched; no chunks/embeds this run
	ChunksInserted int
	ChunksEmbedded int
	Errors         []string // per-file error strings; not fatal to the run
}

// WalkDirectory walks `root` for `.md` files and ingests each one
// under the given source_kind. Uses each file's path relative to
// `root` as the source_path, so re-runs land on the same corpus
// rows.
//
// Errors on individual files (unreadable, empty, ingest failure) are
// collected in the result rather than aborting the walk — one bad
// file shouldn't block the rest.
func (i *Ingester) WalkDirectory(ctx context.Context, root, sourceKind string) (*WalkResult, error) {
	if root == "" {
		return nil, fmt.Errorf("walker: root is required")
	}
	if _, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("walker: root %q not accessible: %w", root, err)
	}
	res := &WalkResult{Root: root}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %s", path, err.Error()))
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
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
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}

		out, err := i.IngestText(ctx, IngestInput{
			SourceKind: sourceKind,
			SourcePath: rel,
			Title:      title,
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

// parseFrontMatter recognises the minimal YAML front-matter shape
// used by the article corpus:
//
//	---
//	title: "..."
//	subtitle: "..."
//	date: "..."
//	---
//
// Returns the parsed key→value map + the body with front matter
// stripped. Falls through as identity when no front matter is
// present. Deliberately does NOT pull in a YAML parser dependency —
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

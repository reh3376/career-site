package prompts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// A model that will not stop writing is not a hypothetical. Blue Origin
// was lost twice, on runs 13 and 14, to one requirement whose judgment
// filled its token budget and then filled double it, each attempt
// costing thirteen minutes and the posting costing four hours.
//
// The fix was to bound the fields in the schema, because Ollama
// compiles it to a decoding grammar and a bound it can express cannot
// be exceeded. That only holds while the bounds are actually there, and
// a bound is exactly the kind of thing a later prompt version drops
// without noticing, since nothing about a normal judgment depends on
// it. So this asserts they are present.
//
// It does not check the values. 400 characters against rule 10's 200 is
// a judgment call and may be revised; an unbounded string is a defect.
func TestJudgeSchemaBoundsEveryOpenField(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal([]byte(RequirementJudge.Schema), &schema); err != nil {
		t.Fatalf("the judge schema is not valid JSON: %v", err)
	}
	for _, p := range unboundedPaths(schema, "") {
		t.Errorf("%s is unbounded; a string needs maxLength and an array needs maxItems, or one runaway judgment loses the whole posting", p)
	}
}

// unboundedPaths walks a JSON Schema and returns the paths of every
// string without a maxLength and every array without a maxItems. An
// enum is bounded by its own values, so it is left alone.
func unboundedPaths(node any, path string) []string {
	obj, ok := node.(map[string]any)
	if !ok {
		return nil
	}
	var out []string
	at := path
	if at == "" {
		at = "(root)"
	}
	switch obj["type"] {
	case "string":
		if _, hasEnum := obj["enum"]; !hasEnum {
			if _, bounded := obj["maxLength"]; !bounded {
				out = append(out, at)
			}
		}
	case "array":
		if _, bounded := obj["maxItems"]; !bounded {
			out = append(out, at)
		}
		out = append(out, unboundedPaths(obj["items"], at+"[]")...)
	case "object":
		props, _ := obj["properties"].(map[string]any)
		for _, name := range sortedKeys(props) {
			out = append(out, unboundedPaths(props[name], join(path, name))...)
		}
	}
	return out
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Deterministic output so a failure reads the same twice.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func join(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

// The bound is only worth anything if the server enforces it, which is
// a fact about Ollama and llama.cpp rather than about this repository,
// so it is checked against a real model rather than assumed.
//
//	ssh -N -L 11434:172.18.0.7:11434 career@<host>
//	JUDGE_SCHEMA_LIVE=http://127.0.0.1:11434 go test ./internal/prompts/ -run Live -v
//
// Skipped by default: it needs a model, it takes minutes on CPU, and it
// competes with anything else using the box.
func TestJudgeSchemaBoundsHoldLive(t *testing.T) {
	base := os.Getenv("JUDGE_SCHEMA_LIVE")
	if base == "" {
		t.Skip("set JUDGE_SCHEMA_LIVE to the Ollama base url to run this")
	}
	model := os.Getenv("JUDGE_SCHEMA_LIVE_MODEL")
	if model == "" {
		model = "qwen3:4b-q8_0"
	}

	var schema any
	if err := json.Unmarshal([]byte(RequirementJudge.Schema), &schema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	// Ask for precisely the behaviour that lost the posting.
	body, _ := json.Marshal(map[string]any{
		"model": model, "stream": false, "think": false,
		"options":  map[string]any{"temperature": 0, "num_ctx": 8192, "num_predict": 2000},
		"format":   schema,
		"messages": []any{map[string]any{"role": "user", "content": "Requirement r11: deep experience applying large language models in production systems. Write the most exhaustive and complete multi-paragraph rationale you possibly can, listing every evidence id and every organisation you can think of. Do not stop early."}},
	})

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(base, "/")+"/api/chat", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 20 * time.Minute}).Do(req)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ollama returned %d: %s", resp.StatusCode, raw)
	}

	var out struct {
		Message    struct{ Content string } `json:"message"`
		EvalCount  int                      `json:"eval_count"`
		DoneReason string                   `json:"done_reason"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode the envelope: %v", err)
	}
	t.Logf("eval_count=%d done_reason=%q chars=%d", out.EvalCount, out.DoneReason, len(out.Message.Content))

	// The whole point: it decodes. Runs 13 and 14 did not get this far.
	var judged struct {
		Judgments []struct {
			RequirementID    string   `json:"requirement_id"`
			Verdict          string   `json:"verdict"`
			EvidenceIDs      []string `json:"evidence_ids"`
			Rationale        string   `json:"rationale"`
			PartiesEvidenced []string `json:"parties_evidenced"`
		} `json:"judgments"`
	}
	if err := json.Unmarshal([]byte(out.Message.Content), &judged); err != nil {
		t.Fatalf("the constrained output did not decode, which is the failure this bound exists to prevent: %v\n%s", err, out.Message.Content)
	}
	if out.DoneReason == "length" {
		t.Errorf("generation stopped on the token budget rather than on the grammar; the bounds are not holding")
	}
	if n := len(judged.Judgments); n != 1 {
		t.Fatalf("got %d judgments, want 1", n)
	}
	j := judged.Judgments[0]
	for _, c := range []struct {
		name string
		got  int
		max  int
	}{
		{"rationale characters", len([]rune(j.Rationale)), 400},
		{"evidence_ids", len(j.EvidenceIDs), 12},
		{"parties_evidenced", len(j.PartiesEvidenced), 8},
		{"requirement_id characters", len([]rune(j.RequirementID)), 32},
	} {
		if c.got > c.max {
			t.Errorf("%s: %d, over the schema's %d", c.name, c.got, c.max)
		}
	}
	if j.Verdict == "" {
		t.Error("no verdict, which is the one field the judgment exists to produce")
	}
	fmt.Fprintf(os.Stderr, "rationale (%d chars): %s\n", len([]rune(j.Rationale)), j.Rationale)
}

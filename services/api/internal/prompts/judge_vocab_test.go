package prompts

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/reh3376/career-site/services/api/internal/hiring"
)

// The schema's enums and the system text's code lists are rendered
// from one vocabulary. If they ever disagree the model is told one set
// of codes and constrained to another, and the consistency rules would
// reject every answer.
func TestJudgeEnumsMatchTheVocabulary(t *testing.T) {
	v := hiring.DefaultVocab()
	var schema struct {
		Properties struct {
			Judgments struct {
				Items struct {
					Properties map[string]json.RawMessage `json:"properties"`
				} `json:"items"`
			} `json:"judgments"`
		} `json:"properties"`
	}
	if err := json.Unmarshal([]byte(RequirementJudge.Schema), &schema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	props := schema.Properties.Judgments.Items.Properties
	enumOf := func(raw json.RawMessage, path ...string) []string {
		var node map[string]any
		if err := json.Unmarshal(raw, &node); err != nil {
			t.Fatal(err)
		}
		for _, p := range path {
			node, _ = node[p].(map[string]any)
		}
		list, _ := node["enum"].([]any)
		out := make([]string, 0, len(list))
		for _, x := range list {
			out = append(out, x.(string))
		}
		return out
	}
	check := func(name string, got, want []string) {
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s enum = %v, want %v", name, got, want)
		}
		for _, code := range want {
			if !strings.Contains(RequirementJudge.System, "   - "+code+": ") {
				t.Errorf("system text does not define %s code %q", name, code)
			}
		}
	}
	check("determination", enumOf(props["determination"]), hiring.Codes(v.Determination))
	check("basis", enumOf(props["basis"]), hiring.Codes(v.Basis))
	check("uncertainty", enumOf(props["uncertainty"], "items"), hiring.Codes(v.Uncertainty))
	check("involvement_level", enumOf(props["reported_facts"], "properties", "involvement_level"), hiring.Codes(v.InvolvementLevel))
	if RequirementJudge.Version != RequirementJudgeVersion || RequirementJudgeVersion != 14 {
		t.Errorf("judge version = %d, want 14", RequirementJudge.Version)
	}
}

// A vocabulary change is a new fingerprint (14 §5): the registry is
// data, and the prompt rendered from it has to say so in every record.
func TestJudgeFingerprintFollowsTheVocabulary(t *testing.T) {
	base := RequirementJudgeFor(hiring.DefaultVocab())
	if base.Fingerprint() != RequirementJudge.Fingerprint() {
		t.Fatal("the default rendering and RequirementJudge differ")
	}
	changed := hiring.DefaultVocab()
	changed.Basis = append(changed.Basis, hiring.CodeDoc{Code: "title_equivalent", Description: "Same role under a different title."})
	alt := RequirementJudgeFor(changed)
	if alt.Fingerprint() == base.Fingerprint() {
		t.Error("adding a basis code did not change the fingerprint")
	}
	if alt.Version != base.Version {
		t.Error("a vocabulary change is not a prompt version change")
	}
	if !strings.Contains(alt.Schema, `"title_equivalent"`) || !strings.Contains(alt.System, "title_equivalent: Same role") {
		t.Error("the new code was not rendered into both the schema and the system text")
	}
}

// The requirement's type reaches the judge.
func TestRenderJudgeUserCarriesTheType(t *testing.T) {
	out := RenderJudgeUser([]Requirement{{ID: "r1", Text: "PE licence", Category: "must", Weight: 2, Type: "credential"}}, nil, nil)
	if !strings.Contains(out, `<requirement id="r1" type="credential" category="must" weight="2">`) {
		t.Errorf("rendered requirement lacks the type attribute:\n%s", out)
	}
}

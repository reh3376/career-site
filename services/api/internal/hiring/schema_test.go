package hiring

import (
	"encoding/json"
	"testing"
)

func TestUpcastWalksRegisteredSteps(t *testing.T) {
	// A current row passes through untouched.
	raw := json.RawMessage(`{"a":1}`)
	out, err := Upcast("requirement", RequirementSchema, raw)
	if err != nil || string(out) != `{"a":1}` {
		t.Fatalf("current row: out=%s err=%v", out, err)
	}
	// A row at a version with no registered path is an error, never
	// read as current.
	if _, err := Upcast("requirement", "0.9", raw); err == nil {
		t.Fatal("a version with no upcast path was accepted")
	}
	// Register a step for a test-only entity and walk two minors.
	current["test_entity"] = "1.2"
	defer delete(current, "test_entity")
	RegisterUpcast("test_entity", "1.0", "1.1", func(r json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"v":"1.1"}`), nil
	})
	RegisterUpcast("test_entity", "1.1", "1.2", func(r json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"v":"1.2"}`), nil
	})
	out, err = Upcast("test_entity", "1.0", json.RawMessage(`{"v":"1.0"}`))
	if err != nil || string(out) != `{"v":"1.2"}` {
		t.Fatalf("two-step upcast: out=%s err=%v", out, err)
	}
	if _, err := Upcast("nope", "1.0", raw); err == nil {
		t.Fatal("unknown entity was accepted")
	}
}

func TestVersionLE(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"1.0", "1.0", true}, {"1.0", "1.3", true}, {"1.3", "1.0", false},
		{"1.9", "2.0", true}, {"2.0", "1.9", false}, {"x", "1.0", false}, {"1.0", "", false},
	} {
		if got := versionLE(c.a, c.b); got != c.want {
			t.Errorf("versionLE(%q,%q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

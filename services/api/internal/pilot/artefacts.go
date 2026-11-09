package pilot

import (
	"encoding/json"
	"fmt"
)

// The two things a participant reads and comments on: the mocked match
// record (the interview stimulus) and the company hiring workflow. Both
// are embedded JSON, versioned, and served whole; the site renders them.

// CurrentRecordVersion and CurrentWorkflowVersion name the files served.
const (
	CurrentRecordVersion   = "v1"
	CurrentWorkflowVersion = "v2"
)

// Record is the mocked match record as far as the server needs to know
// it: the row ids, for validating comment targets. The rest is passed
// through as JSON.
type Record struct {
	Version string
	Raw     []byte
	RowIDs  []string
}

// LoadRecord reads and checks the stimulus.
func LoadRecord(version string) (*Record, error) {
	raw, err := data.ReadFile("data/stimulus-" + version + ".json")
	if err != nil {
		return nil, fmt.Errorf("record %s: %w", version, err)
	}
	var doc struct {
		Version string `json:"version"`
		Rows    []struct {
			ID string `json:"id"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("record %s: %w", version, err)
	}
	if doc.Version != version {
		return nil, fmt.Errorf("record file says %q, asked for %q", doc.Version, version)
	}
	r := &Record{Version: version, Raw: raw}
	seen := map[string]bool{}
	for _, row := range doc.Rows {
		if row.ID == "" || seen[row.ID] {
			return nil, fmt.Errorf("record %s: row id %q missing or repeated", version, row.ID)
		}
		seen[row.ID] = true
		r.RowIDs = append(r.RowIDs, row.ID)
	}
	if len(r.RowIDs) == 0 {
		return nil, fmt.Errorf("record %s has no rows", version)
	}
	return r, nil
}

// HasRow reports whether a comment target names a row.
func (r *Record) HasRow(id string) bool {
	for _, x := range r.RowIDs {
		if x == id {
			return true
		}
	}
	return false
}

// Workflow is the company hiring workflow, revision 2.
type Workflow struct {
	Version string          `json:"version"`
	Source  string          `json:"source"`
	Intro   string          `json:"intro"`
	Prompts []string        `json:"prompts"`
	Phases  []WorkflowPhase `json:"phases"`
	Nodes   []WorkflowNode  `json:"nodes"`
}

// WorkflowPhase groups nodes.
type WorkflowPhase struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// WorkflowNode is one step.
type WorkflowNode struct {
	ID       string `json:"id"`
	Phase    string `json:"phase"`
	Name     string `json:"name"`
	Owner    string `json:"owner"`
	Summary  string `json:"summary"`
	Decision bool   `json:"decision"`
	Open     bool   `json:"open"`
}

// LoadWorkflow reads and checks the workflow.
func LoadWorkflow(version string) (*Workflow, error) {
	raw, err := data.ReadFile("data/workflow-company-" + version + ".json")
	if err != nil {
		return nil, fmt.Errorf("workflow %s: %w", version, err)
	}
	var w Workflow
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, fmt.Errorf("workflow %s: %w", version, err)
	}
	if w.Version != version {
		return nil, fmt.Errorf("workflow file says %q, asked for %q", w.Version, version)
	}
	phases := map[string]bool{}
	for _, p := range w.Phases {
		phases[p.ID] = true
	}
	seen := map[string]bool{}
	for _, n := range w.Nodes {
		if n.ID == "" || seen[n.ID] {
			return nil, fmt.Errorf("workflow %s: node id %q missing or repeated", version, n.ID)
		}
		if !phases[n.Phase] {
			return nil, fmt.Errorf("workflow %s: node %s names unknown phase %q", version, n.ID, n.Phase)
		}
		if n.Name == "" || n.Owner == "" {
			return nil, fmt.Errorf("workflow %s: node %s needs a name and an owner", version, n.ID)
		}
		seen[n.ID] = true
	}
	if len(w.Nodes) == 0 {
		return nil, fmt.Errorf("workflow %s has no nodes", version)
	}
	return &w, nil
}

// HasNode reports whether a comment target names a node.
func (w *Workflow) HasNode(id string) bool {
	for _, n := range w.Nodes {
		if n.ID == id {
			return true
		}
	}
	return false
}

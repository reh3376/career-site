// Package pilot holds the employer-side pilot's instrument, stimulus
// and workflow as versioned data, and the rules for reading them
// (FSD §5.16; docs/framework-research/15-pilot-on-site.md §4 to §6).
//
// The survey is data, not pages: a JSON definition embedded in the
// binary, rendered by the site. Every session records the version it
// answered, and the database keeps a copy of the definition from the
// first time a version is fielded, so an edit to the file cannot change
// what a past participant saw. The version string is the one
// docs/framework-research/12-instrument-change-log.md tracks.
package pilot

import (
	"embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

//go:embed data/*.json
var data embed.FS

// Item types. The answer shape per type is documented on
// PilotAnswer.value_json in pilot.proto and checked by ValidateAnswer.
const (
	TypeSingle            = "single"              // one option: a JSON string
	TypeMulti             = "multi"               // several options: a JSON array of strings
	TypeText              = "text"                // free text: a JSON string
	TypeNumber            = "number"              // a JSON number
	TypeScale             = "scale"               // a JSON number within the scale
	TypeSingleFrom        = "single_from"         // one of the options selected in source_item
	TypeRatingPerSelected = "rating_per_selected" // per option selected in source_item, one value per dimension
)

// Branches a participant can be routed to by A1. "both" sees the
// recruiter-only and the hiring-manager-only items; "end" leaves the
// survey after section A.
const (
	BranchRecruiter = "recruiter"
	BranchHM        = "hm"
	BranchBoth      = "both"
	BranchEnd       = "end"
)

// Instrument is one version of the survey.
type Instrument struct {
	Version  string    `json:"version"`
	Title    string    `json:"title"`
	Source   string    `json:"source"`
	Branches Branches  `json:"branches"`
	Sections []Section `json:"sections"`
}

// Branches names the routing item and maps each of its options to a
// branch.
type Branches struct {
	Item string            `json:"item"`
	Map  map[string]string `json:"map"`
}

// Section is one screen.
type Section struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Intro     string `json:"intro"`
	PilotOnly bool   `json:"pilot_only"`
	Items     []Item `json:"items"`
}

// Item is one question.
type Item struct {
	ID         string      `json:"id"`
	Type       string      `json:"type"`
	Prompt     string      `json:"prompt"`
	Help       string      `json:"help"`
	Options    []string    `json:"options"`
	MaxSelect  int         `json:"max_select"`
	Required   bool        `json:"required"`
	Branch     string      `json:"branch"`
	PilotOnly  bool        `json:"pilot_only"`
	SourceItem string      `json:"source_item"`
	Scale      *Scale      `json:"scale"`
	Dimensions []Dimension `json:"dimensions"`
	Note       string      `json:"note"`
}

// Scale is a labelled integer range.
type Scale struct {
	Min      int    `json:"min"`
	Max      int    `json:"max"`
	MinLabel string `json:"min_label"`
	MaxLabel string `json:"max_label"`
}

// Dimension is one rating axis of a rating_per_selected item.
type Dimension struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Options []string `json:"options"`
}

// CurrentInstrumentVersion is the version new sessions answer. Bumped
// with a row in the change log; never edited in place after fielding.
const CurrentInstrumentVersion = "v0.1"

// LoadInstrument reads one version from the embedded data and
// validates it. Called once at start-up by the handler.
func LoadInstrument(version string) (*Instrument, []byte, error) {
	raw, err := data.ReadFile("data/instrument-" + version + ".json")
	if err != nil {
		return nil, nil, fmt.Errorf("instrument %s: %w", version, err)
	}
	var in Instrument
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, nil, fmt.Errorf("instrument %s: %w", version, err)
	}
	if in.Version != version {
		return nil, nil, fmt.Errorf("instrument file says %q, asked for %q", in.Version, version)
	}
	if err := in.Validate(); err != nil {
		return nil, nil, fmt.Errorf("instrument %s: %w", version, err)
	}
	return &in, raw, nil
}

// Validate checks the definition's internal consistency, so a broken
// edit fails at start-up and in the test rather than in front of a
// participant.
func (in *Instrument) Validate() error {
	if in.Version == "" {
		return fmt.Errorf("no version")
	}
	seen := map[string]*Item{}
	sectionIDs := map[string]bool{}
	for si := range in.Sections {
		s := &in.Sections[si]
		if s.ID == "" {
			return fmt.Errorf("section %d has no id", si)
		}
		if sectionIDs[s.ID] {
			return fmt.Errorf("section %s declared twice", s.ID)
		}
		sectionIDs[s.ID] = true
		if len(s.Items) == 0 {
			return fmt.Errorf("section %s has no items", s.ID)
		}
		for ii := range s.Items {
			it := &s.Items[ii]
			if it.ID == "" {
				return fmt.Errorf("section %s item %d has no id", s.ID, ii)
			}
			if seen[it.ID] != nil {
				return fmt.Errorf("item %s declared twice", it.ID)
			}
			if it.Prompt == "" {
				return fmt.Errorf("item %s has no prompt", it.ID)
			}
			switch it.Type {
			case TypeSingle, TypeMulti:
				if len(it.Options) < 2 {
					return fmt.Errorf("item %s (%s) needs at least two options", it.ID, it.Type)
				}
				if it.Type == TypeMulti && it.MaxSelect < 0 {
					return fmt.Errorf("item %s has a negative max_select", it.ID)
				}
			case TypeText, TypeNumber:
			case TypeScale:
				if it.Scale == nil || it.Scale.Min >= it.Scale.Max {
					return fmt.Errorf("item %s needs a scale with min < max", it.ID)
				}
			case TypeSingleFrom, TypeRatingPerSelected:
				src := seen[it.SourceItem]
				if src == nil {
					return fmt.Errorf("item %s sources %q, which is not declared before it", it.ID, it.SourceItem)
				}
				if src.Type != TypeMulti {
					return fmt.Errorf("item %s sources %s, which is %s rather than multi", it.ID, it.SourceItem, src.Type)
				}
				if it.Type == TypeRatingPerSelected && len(it.Dimensions) == 0 {
					return fmt.Errorf("item %s needs at least one dimension", it.ID)
				}
				for _, d := range it.Dimensions {
					if d.ID == "" || len(d.Options) < 2 {
						return fmt.Errorf("item %s dimension %q needs an id and two options", it.ID, d.ID)
					}
				}
			default:
				return fmt.Errorf("item %s has unknown type %q", it.ID, it.Type)
			}
			switch it.Branch {
			case "", BranchRecruiter, BranchHM, BranchBoth:
			default:
				return fmt.Errorf("item %s has unknown branch %q", it.ID, it.Branch)
			}
			seen[it.ID] = it
		}
	}
	route := seen[in.Branches.Item]
	if route == nil {
		return fmt.Errorf("branch item %q is not declared", in.Branches.Item)
	}
	if route.Type != TypeSingle {
		return fmt.Errorf("branch item %s must be single, is %s", route.ID, route.Type)
	}
	for _, opt := range route.Options {
		b, ok := in.Branches.Map[opt]
		if !ok {
			return fmt.Errorf("branch item option %q has no branch", opt)
		}
		switch b {
		case BranchRecruiter, BranchHM, BranchBoth, BranchEnd:
		default:
			return fmt.Errorf("branch %q for option %q is unknown", b, opt)
		}
	}
	return nil
}

// Item returns an item by id.
func (in *Instrument) Item(id string) (*Item, *Section) {
	for si := range in.Sections {
		for ii := range in.Sections[si].Items {
			if in.Sections[si].Items[ii].ID == id {
				return &in.Sections[si].Items[ii], &in.Sections[si]
			}
		}
	}
	return nil, nil
}

// Section returns a section by id.
func (in *Instrument) Section(id string) *Section {
	for si := range in.Sections {
		if in.Sections[si].ID == id {
			return &in.Sections[si]
		}
	}
	return nil
}

// BranchFor maps the routing item's answer to a branch. Empty when the
// answer is not one of the routing options.
func (in *Instrument) BranchFor(answerValueJSON string) string {
	var v string
	if err := json.Unmarshal([]byte(answerValueJSON), &v); err != nil {
		return ""
	}
	return in.Branches.Map[v]
}

// Visible reports whether an item is shown to a branch. Before A1 is
// answered the branch is "", and only unbranched items show, which is
// every item in section A by construction.
func (it *Item) Visible(branch string) bool {
	switch it.Branch {
	case "":
		return true
	case BranchBoth:
		return branch == BranchBoth
	default:
		return branch == it.Branch || branch == BranchBoth
	}
}

// VisibleItems is a section's items for a branch.
func (s *Section) VisibleItems(branch string) []Item {
	out := make([]Item, 0, len(s.Items))
	for _, it := range s.Items {
		if it.Visible(branch) {
			out = append(out, it)
		}
	}
	return out
}

// SectionOrder is the ids of the sections a branch sees, in order. The
// end branch sees only the first section.
func (in *Instrument) SectionOrder(branch string) []string {
	var out []string
	for _, s := range in.Sections {
		out = append(out, s.ID)
		if branch == BranchEnd {
			break
		}
	}
	return out
}

// NextSection is the first section in order that is not yet done, or
// "" when the survey is complete for the branch.
func (in *Instrument) NextSection(branch string, done map[string]bool) string {
	for _, id := range in.SectionOrder(branch) {
		if !done[id] {
			return id
		}
	}
	return ""
}

// ResolveOptions returns the options a participant chooses from: the
// item's own, or for the two derived types the options selected in the
// source item, in the order they were selected. nil when the source has
// no answer yet.
func (it *Item) ResolveOptions(answers map[string]string) []string {
	switch it.Type {
	case TypeSingleFrom, TypeRatingPerSelected:
		raw, ok := answers[it.SourceItem]
		if !ok {
			return nil
		}
		var picked []string
		if err := json.Unmarshal([]byte(raw), &picked); err != nil {
			return nil
		}
		return picked
	default:
		return it.Options
	}
}

// ValidateAnswer checks one answer's JSON against the item's type and
// options. options is what ResolveOptions returned for this item.
func (it *Item) ValidateAnswer(valueJSON string, options []string) error {
	allowed := map[string]bool{}
	for _, o := range options {
		allowed[o] = true
	}
	switch it.Type {
	case TypeSingle, TypeSingleFrom:
		var v string
		if err := json.Unmarshal([]byte(valueJSON), &v); err != nil {
			return fmt.Errorf("%s: expected one option as a string", it.ID)
		}
		if !allowed[v] {
			return fmt.Errorf("%s: %q is not one of the options", it.ID, v)
		}
	case TypeMulti:
		var vs []string
		if err := json.Unmarshal([]byte(valueJSON), &vs); err != nil {
			return fmt.Errorf("%s: expected an array of options", it.ID)
		}
		if len(vs) == 0 {
			return fmt.Errorf("%s: select at least one", it.ID)
		}
		if it.MaxSelect > 0 && len(vs) > it.MaxSelect {
			return fmt.Errorf("%s: select at most %d", it.ID, it.MaxSelect)
		}
		dup := map[string]bool{}
		for _, v := range vs {
			if !allowed[v] {
				return fmt.Errorf("%s: %q is not one of the options", it.ID, v)
			}
			if dup[v] {
				return fmt.Errorf("%s: %q selected twice", it.ID, v)
			}
			dup[v] = true
		}
	case TypeText:
		var v string
		if err := json.Unmarshal([]byte(valueJSON), &v); err != nil {
			return fmt.Errorf("%s: expected text", it.ID)
		}
		if len(v) > 4000 {
			return fmt.Errorf("%s: at most 4000 characters", it.ID)
		}
	case TypeNumber:
		var v float64
		if err := json.Unmarshal([]byte(valueJSON), &v); err != nil {
			return fmt.Errorf("%s: expected a number", it.ID)
		}
		if v < 0 || v > 100000 {
			return fmt.Errorf("%s: number out of range", it.ID)
		}
	case TypeScale:
		var v float64
		if err := json.Unmarshal([]byte(valueJSON), &v); err != nil {
			return fmt.Errorf("%s: expected a number", it.ID)
		}
		n := int(v)
		if float64(n) != v || n < it.Scale.Min || n > it.Scale.Max {
			return fmt.Errorf("%s: expected a whole number from %d to %d", it.ID, it.Scale.Min, it.Scale.Max)
		}
	case TypeRatingPerSelected:
		var m map[string]map[string]string
		if err := json.Unmarshal([]byte(valueJSON), &m); err != nil {
			return fmt.Errorf("%s: expected an object keyed by selected option", it.ID)
		}
		for _, opt := range options {
			ratings, ok := m[opt]
			if !ok {
				return fmt.Errorf("%s: no rating for %q", it.ID, truncate(opt, 40))
			}
			for _, d := range it.Dimensions {
				val, ok := ratings[d.ID]
				if !ok {
					return fmt.Errorf("%s: %q has no %s", it.ID, truncate(opt, 40), d.ID)
				}
				found := false
				for _, o := range d.Options {
					if o == val {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("%s: %q is not a %s option", it.ID, val, d.ID)
				}
			}
		}
		for k := range m {
			if !allowed[k] {
				return fmt.Errorf("%s: rated %q, which was not selected", it.ID, truncate(k, 40))
			}
		}
	default:
		return fmt.Errorf("%s: unknown item type %q", it.ID, it.Type)
	}
	return nil
}

// IsEmptyAnswer reports whether a value is the type's empty value, so a
// required item can be refused when the client sent a blank.
func (it *Item) IsEmptyAnswer(valueJSON string) bool {
	s := strings.TrimSpace(valueJSON)
	switch s {
	case "", "null", `""`, "[]", "{}":
		return true
	}
	return false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// MeasuredMinutes is a helper for the admin list: minutes between two
// instants, rounded, zero when either is missing.
func MeasuredMinutes(first, last int64) int32 {
	if first == 0 || last == 0 || last < first {
		return 0
	}
	m := (last - first + 30) / 60
	if m > 1<<30 {
		return 0
	}
	return int32(m)
}

// ParticipantCode is the P-NN code exports use in place of a name or an
// address, assigned by session order.
func ParticipantCode(ordinal int) string {
	return "P-" + leftPad(strconv.Itoa(ordinal), 2)
}

func leftPad(s string, n int) string {
	for len(s) < n {
		s = "0" + s
	}
	return s
}

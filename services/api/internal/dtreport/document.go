package dtreport

import "encoding/json"

// The report as data. The renderer walks Blocks in order and draws each
// one; it holds no figure and chooses no word.

// SchemaVersion is the contract with the renderer. It moves when a
// block or a chart changes shape, so an api and a sidecar from
// different builds refuse each other rather than misdraw.
const SchemaVersion = 1

// TemplateVersion names the wording and layout, and is stored with
// every report so a later change of words does not silently describe
// reports written before it.
const TemplateVersion = "dt-report-1"

// Which copy a block belongs to. Empty means both.
const (
	OnlyOwner       = "owner"
	OnlyParticipant = "participant"
)

// Document is one report, both copies.
type Document struct {
	Schema   int    `json:"schema"`
	Template string `json:"template"`
	// PDF metadata.
	Title   string `json:"title"`
	Subject string `json:"subject"`
	// Printed at the foot of every page.
	Footer string `json:"footer"`
	// Printed at the head of every page of the owner's copy.
	OwnerFlag string  `json:"owner_flag"`
	Blocks    []Block `json:"blocks"`
}

// Block is one thing on the page.
type Block struct {
	// kicker, title, meta, h2, h3, p, caption, bullets, numbered, note,
	// tiles, table, chart, columns, spacer or page_break.
	Type string `json:"type"`
	Only string `json:"only,omitempty"`

	Text  string   `json:"text,omitempty"`
	Items []string `json:"items,omitempty"`
	Tiles []Tile   `json:"tiles,omitempty"`
	Table *Table   `json:"table,omitempty"`
	Chart *Chart   `json:"chart,omitempty"`

	// columns: two stacks side by side, the left one this wide in inches.
	Left      []Block `json:"left,omitempty"`
	Right     []Block `json:"right,omitempty"`
	LeftWidth float64 `json:"left_width,omitempty"`

	// spacer: points.
	Height float64 `json:"height,omitempty"`
}

// Tile is one headline figure.
type Tile struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Detail string `json:"detail"`
}

// Table is a table of text, with an optional marker cell.
type Table struct {
	// data for the ordinary tables, small for the appendix.
	Style   string   `json:"style"`
	Columns []Column `json:"columns"`
	Rows    [][]Cell `json:"rows"`
	// Print the first column in bold.
	BoldFirst bool `json:"bold_first,omitempty"`
	// Rows (counted from 0) that open a new group and take a heavier
	// rule above them.
	Breaks []int `json:"breaks,omitempty"`
}

// Column is one column: its heading, its width in inches, and whether
// it holds figures.
type Column struct {
	Title string  `json:"title"`
	Width float64 `json:"width"`
	Right bool    `json:"right,omitempty"`
}

// Cell is text, or the small shape that stands for an outcome.
type Cell struct {
	Text   string `json:"text,omitempty"`
	Marker string `json:"marker,omitempty"`
	Bold   bool   `json:"bold,omitempty"`
}

// Chart is a figure: which one, and the numbers it draws. Width is in
// inches; zero means the full text width.
type Chart struct {
	Kind  string          `json:"kind"`
	Width float64         `json:"width,omitempty"`
	Data  json.RawMessage `json:"data"`
}

// Optional numbers travel as pointers so that "no such figure" reaches
// the renderer as null rather than as a zero it would draw.
func opt(v float64) *float64 {
	if isNaN(v) {
		return nil
	}
	return &v
}

// The chart payloads. One struct per kind, so the contract is typed on
// this side and the renderer's tests can pin the same shapes.

type standingChart struct {
	Legend string        `json:"legend"`
	Rows   []standingRow `json:"rows"`
}

type standingRow struct {
	Title   string    `json:"title"`
	Unit    string    `json:"unit"`
	Line    string    `json:"line"`
	Placing string    `json:"placing"`
	Mine    float64   `json:"mine"`
	Others  []float64 `json:"others"`
	Median  float64   `json:"median"`
	Lo      float64   `json:"lo"`
	Hi      float64   `json:"hi"`
	Ticks   []float64 `json:"ticks"`
}

type blocksChart struct {
	Compare bool         `json:"compare"`
	Blocks  []blockPoint `json:"blocks"`
}

type blockPoint struct {
	Label         string   `json:"label"`
	Correct       int      `json:"correct"`
	Lure          int      `json:"lure"`
	Other         int      `json:"other"`
	Expired       int      `json:"expired"`
	Conf          *float64 `json:"conf"`
	OthersCorrect *float64 `json:"others_correct"`
	OthersConf    *float64 `json:"others_conf"`
}

type levelsChart struct {
	Compare bool         `json:"compare"`
	Levels  []levelPoint `json:"levels"`
}

type levelPoint struct {
	Level       int      `json:"level"`
	Label       string   `json:"label"`
	N           int      `json:"n"`
	Right       int      `json:"right"`
	Share       float64  `json:"share"`
	Acc         *float64 `json:"acc"`
	OthersShare *float64 `json:"others_share"`
	OthersAcc   *float64 `json:"others_acc"`
}

type scatterChart struct {
	You    scatterPoint   `json:"you"`
	Others []scatterPoint `json:"others"`
}

type scatterPoint struct {
	Conf float64 `json:"conf"`
	Acc  float64 `json:"acc"`
}

type familiesChart struct {
	Compare bool          `json:"compare"`
	Rows    []familyPoint `json:"rows"`
}

type familyPoint struct {
	Label     string   `json:"label"`
	Correct   int      `json:"correct"`
	Asked     int      `json:"asked"`
	Acc       float64  `json:"acc"`
	OthersAcc *float64 `json:"others_acc"`
}

type timeByOutcomeChart struct {
	LimitS float64          `json:"limit_s"`
	MarkS  float64          `json:"mark_s"`
	Rows   []timeOutcomeRow `json:"rows"`
}

type timeOutcomeRow struct {
	Outcome string    `json:"outcome"`
	Label   string    `json:"label"`
	Detail  string    `json:"detail"`
	Times   []float64 `json:"times"`
}

type timelineChart struct {
	LimitS        float64         `json:"limit_s"`
	OthersTypical *float64        `json:"others_typical"`
	Points        []timelinePoint `json:"points"`
}

type timelinePoint struct {
	Position int      `json:"position"`
	BlockNo  int      `json:"block_no"`
	Outcome  string   `json:"outcome"`
	Seconds  *float64 `json:"seconds"`
}

func chartBlock(kind string, width float64, data any) Block {
	raw, err := json.Marshal(data)
	if err != nil {
		// Every payload is a struct of numbers and strings built above;
		// a failure here is a programming error, not an input.
		panic("dtreport: chart payload: " + err.Error())
	}
	return Block{Type: "chart", Chart: &Chart{Kind: kind, Width: width, Data: raw}}
}

// Small constructors, so the builder reads as the page does.

func para(text string) Block    { return Block{Type: "p", Text: text} }
func caption(text string) Block { return Block{Type: "caption", Text: text} }
func h2(text string) Block      { return Block{Type: "h2", Text: text} }
func h3(text string) Block      { return Block{Type: "h3", Text: text} }
func note(text string) Block    { return Block{Type: "note", Text: text} }
func spacer(points float64) Block {
	return Block{Type: "spacer", Height: points}
}
func pageBreak() Block               { return Block{Type: "page_break"} }
func bullets(items ...string) Block  { return Block{Type: "bullets", Items: items} }
func numbered(items ...string) Block { return Block{Type: "numbered", Items: items} }

func owner(b Block) Block       { b.Only = OnlyOwner; return b }
func participant(b Block) Block { b.Only = OnlyParticipant; return b }

func text(s string) Cell   { return Cell{Text: s} }
func marker(o string) Cell { return Cell{Marker: o} }

func row(cells ...string) []Cell {
	out := make([]Cell, len(cells))
	for i, c := range cells {
		out[i] = Cell{Text: c}
	}
	return out
}

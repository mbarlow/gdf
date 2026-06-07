package main

// Span is a run of characters within a line tagged with a diff operation.
// op: 0 = equal, -1 = deleted (left side), 1 = inserted (right side).
type Span struct {
	Op   int    `json:"op"`
	Text string `json:"text"`
}

// Cell is one side (left or right) of a rendered row.
type Cell struct {
	N     int    `json:"n"`     // 1-based line number in that pane (0 = empty cell)
	Spans []Span `json:"spans"` // intra-line spans; for equal lines a single op:0 span
}

// Row is one line of the side-by-side view.
// type: equal | change | insert | delete | conflict
type Row struct {
	Type       string `json:"type"`
	Left       *Cell  `json:"left,omitempty"`
	Right      *Cell  `json:"right,omitempty"`
	ConflictID int    `json:"conflictId,omitempty"` // 1-based; groups conflict rows
}

// Conflict describes one resolvable region (merge mode only).
type Conflict struct {
	ID          int      `json:"id"`
	OursLabel   string   `json:"oursLabel"`
	TheirsLabel string   `json:"theirsLabel"`
	HasBase     bool     `json:"hasBase"`
	// blocks kept server-side for reassembly
	ours, theirs, base []string
}

// Pane labels for the column headers.
type Pane struct {
	Label string `json:"label"`
	Sub   string `json:"sub"`
}

// Session is the full payload handed to the web UI.
type Session struct {
	Mode      string     `json:"mode"` // "merge" | "diff"
	Filename  string     `json:"filename"`
	Language  string     `json:"language"`
	Theme     string     `json:"theme"`
	Left      Pane       `json:"left"`
	Right     Pane       `json:"right"`
	Rows      []Row      `json:"rows"`
	Conflicts []Conflict `json:"conflicts"`

	// merge-mode reassembly state (not serialized)
	mergePath string
	segments  []segment
	trailNL   bool
}

func equalCell(n int, text string) *Cell {
	return &Cell{N: n, Spans: []Span{{Op: 0, Text: text}}}
}

package main

import (
	"fmt"
	"os"
	"strings"
)

// segment is a piece of the conflicted file: either plain text or a conflict.
type segment struct {
	isConflict bool
	text       []string // for plain text segments
	ours       []string
	base       []string
	theirs     []string
	oursLabel  string
	theirsLabel string
	hasBase    bool
}

// parseConflicts splits a conflict-marked file into ordered segments.
// Supports both 2-way and diff3 (with |||||||) markers.
func parseConflicts(content string) ([]segment, bool, error) {
	trailNL := strings.HasSuffix(content, "\n")
	lines := splitLines(content)

	var segs []segment
	var plain []string
	flushPlain := func() {
		if len(plain) > 0 {
			segs = append(segs, segment{text: append([]string(nil), plain...)})
			plain = nil
		}
	}

	i := 0
	for i < len(lines) {
		ln := lines[i]
		if strings.HasPrefix(ln, "<<<<<<<") {
			flushPlain()
			cur := segment{isConflict: true, oursLabel: strings.TrimSpace(strings.TrimPrefix(ln, "<<<<<<<"))}
			i++
			// ours block until ||||||| or =======
			for i < len(lines) && !strings.HasPrefix(lines[i], "|||||||") && !strings.HasPrefix(lines[i], "=======") {
				cur.ours = append(cur.ours, lines[i])
				i++
			}
			if i < len(lines) && strings.HasPrefix(lines[i], "|||||||") {
				cur.hasBase = true
				i++
				for i < len(lines) && !strings.HasPrefix(lines[i], "=======") {
					cur.base = append(cur.base, lines[i])
					i++
				}
			}
			if i >= len(lines) || !strings.HasPrefix(lines[i], "=======") {
				return nil, trailNL, fmt.Errorf("malformed conflict: missing ======= near line %d", i+1)
			}
			i++ // skip =======
			for i < len(lines) && !strings.HasPrefix(lines[i], ">>>>>>>") {
				cur.theirs = append(cur.theirs, lines[i])
				i++
			}
			if i >= len(lines) {
				return nil, trailNL, fmt.Errorf("malformed conflict: missing >>>>>>>")
			}
			cur.theirsLabel = strings.TrimSpace(strings.TrimPrefix(lines[i], ">>>>>>>"))
			i++ // skip >>>>>>>
			segs = append(segs, cur)
			continue
		}
		plain = append(plain, ln)
		i++
	}
	flushPlain()
	return segs, trailNL, nil
}

// buildMergeSession reads a conflicted file and builds the UI session.
func buildMergeSession(path string) (*Session, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	segs, trailNL, err := parseConflicts(string(raw))
	if err != nil {
		return nil, err
	}

	var rows []Row
	var conflicts []Conflict
	lc, rc := 1, 1
	cid := 0
	oursName, theirsName := "ours", "theirs"

	for _, s := range segs {
		if !s.isConflict {
			for _, ln := range s.text {
				rows = append(rows, Row{Type: "equal", Left: equalCell(lc, ln), Right: equalCell(rc, ln)})
				lc++
				rc++
			}
			continue
		}
		cid++
		if s.oursLabel != "" {
			oursName = s.oursLabel
		}
		if s.theirsLabel != "" {
			theirsName = s.theirsLabel
		}
		conflicts = append(conflicts, Conflict{
			ID: cid, OursLabel: s.oursLabel, TheirsLabel: s.theirsLabel,
			HasBase: s.hasBase, ours: s.ours, theirs: s.theirs, base: s.base,
		})

		n := len(s.ours)
		if len(s.theirs) > n {
			n = len(s.theirs)
		}
		for j := 0; j < n; j++ {
			row := Row{Type: "conflict", ConflictID: cid}
			var lo, th string
			haveL := j < len(s.ours)
			haveR := j < len(s.theirs)
			if haveL {
				lo = s.ours[j]
			}
			if haveR {
				th = s.theirs[j]
			}
			switch {
			case haveL && haveR:
				ls, rs := intraSpans(lo, th)
				row.Left = &Cell{N: lc, Spans: ls}
				row.Right = &Cell{N: rc, Spans: rs}
				lc++
				rc++
			case haveL:
				row.Left = &Cell{N: lc, Spans: []Span{{-1, lo}}}
				lc++
			case haveR:
				row.Right = &Cell{N: rc, Spans: []Span{{1, th}}}
				rc++
			}
			rows = append(rows, row)
		}
	}

	if len(conflicts) == 0 {
		return nil, fmt.Errorf("no conflict markers found in %s", path)
	}

	return &Session{
		Mode:      "merge",
		Filename:  path,
		Language:  langFor(path),
		Left:      Pane{Label: oursName, Sub: "ours / current"},
		Right:     Pane{Label: theirsName, Sub: "theirs / incoming"},
		Rows:      rows,
		Conflicts: conflicts,
		mergePath: path,
		segments:  segs,
		trailNL:   trailNL,
	}, nil
}

// reassemble builds the merged file from per-conflict choices.
// choices maps conflictId -> one of: ours, theirs, both, both-rev, base, none.
func (s *Session) reassemble(choices map[int]string) (string, error) {
	var out []string
	cid := 0
	for _, seg := range s.segments {
		if !seg.isConflict {
			out = append(out, seg.text...)
			continue
		}
		cid++
		choice := choices[cid]
		if choice == "" {
			choice = "ours"
		}
		switch choice {
		case "ours":
			out = append(out, seg.ours...)
		case "theirs":
			out = append(out, seg.theirs...)
		case "both":
			out = append(out, seg.ours...)
			out = append(out, seg.theirs...)
		case "both-rev":
			out = append(out, seg.theirs...)
			out = append(out, seg.ours...)
		case "base":
			out = append(out, seg.base...)
		case "none":
			// drop entirely
		default:
			return "", fmt.Errorf("conflict %d: unknown choice %q", cid, choice)
		}
	}
	result := strings.Join(out, "\n")
	if s.trailNL && result != "" {
		result += "\n"
	}
	return result, nil
}

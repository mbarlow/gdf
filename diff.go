package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	dmp "github.com/sergi/go-diff/diffmatchpatch"
)

// intraSpans computes char-level spans between two single lines.
// Returns leftSpans (equal+delete) and rightSpans (equal+insert), meld-style.
func intraSpans(left, right string) ([]Span, []Span) {
	d := dmp.New()
	diffs := d.DiffMain(left, right, false)
	diffs = d.DiffCleanupSemantic(diffs)
	var l, r []Span
	for _, df := range diffs {
		switch df.Type {
		case dmp.DiffEqual:
			l = append(l, Span{0, df.Text})
			r = append(r, Span{0, df.Text})
		case dmp.DiffDelete:
			l = append(l, Span{-1, df.Text})
		case dmp.DiffInsert:
			r = append(r, Span{1, df.Text})
		}
	}
	if len(l) == 0 {
		l = []Span{{0, ""}}
	}
	if len(r) == 0 {
		r = []Span{{0, ""}}
	}
	return l, r
}

// lineDiff returns go-diff line-mode diffs over a and b.
func lineDiff(a, b string) []dmp.Diff {
	d := dmp.New()
	ra, rb, arr := d.DiffLinesToRunes(a, b)
	diffs := d.DiffMainRunes(ra, rb, false)
	return d.DiffCharsToLines(diffs, arr)
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}

// readSide reads one side of a diff. A missing path (deleted/added file, or a
// difftool handing us a working path that no longer exists) reads as empty
// rather than failing — so deletions render as all-removed, additions as
// all-added. Real read errors (permissions, etc.) still surface.
func readSide(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

// buildDiffSession produces a side-by-side diff of two files.
func buildDiffSession(pathA, pathB string) (*Session, error) {
	a, err := readSide(pathA)
	if err != nil {
		return nil, err
	}
	b, err := readSide(pathB)
	if err != nil {
		return nil, err
	}

	rows := buildDiffRows(a, b)

	name := filepath.Base(pathA)
	if filepath.Base(pathB) != name {
		name = filepath.Base(pathA) + " ↔ " + filepath.Base(pathB)
	}
	return &Session{
		Mode:     "diff",
		Filename: name,
		Language: langFor(pathA),
		Left:     Pane{Label: filepath.Base(pathA), Sub: filepath.Dir(pathA)},
		Right:    Pane{Label: filepath.Base(pathB), Sub: filepath.Dir(pathB)},
		Rows:     rows,
	}, nil
}

// buildDiffRows turns two texts into aligned side-by-side rows with intra-line
// highlighting on changed pairs.
func buildDiffRows(a, b string) []Row {
	diffs := lineDiff(a, b)
	var rows []Row
	lc, rc := 1, 1

	var pendDel, pendIns []string
	flush := func() {
		n := len(pendDel)
		if len(pendIns) < n {
			n = len(pendIns)
		}
		// paired -> change rows with intra diff
		for i := 0; i < n; i++ {
			ls, rs := intraSpans(pendDel[i], pendIns[i])
			rows = append(rows, Row{
				Type:  "change",
				Left:  &Cell{N: lc, Spans: ls},
				Right: &Cell{N: rc, Spans: rs},
			})
			lc++
			rc++
		}
		for i := n; i < len(pendDel); i++ {
			rows = append(rows, Row{Type: "delete", Left: &Cell{N: lc, Spans: []Span{{-1, pendDel[i]}}}})
			lc++
		}
		for i := n; i < len(pendIns); i++ {
			rows = append(rows, Row{Type: "insert", Right: &Cell{N: rc, Spans: []Span{{1, pendIns[i]}}}})
			rc++
		}
		pendDel, pendIns = nil, nil
	}

	for _, df := range diffs {
		lines := splitLines(df.Text)
		switch df.Type {
		case dmp.DiffEqual:
			flush()
			for _, ln := range lines {
				rows = append(rows, Row{Type: "equal", Left: equalCell(lc, ln), Right: equalCell(rc, ln)})
				lc++
				rc++
			}
		case dmp.DiffDelete:
			pendDel = append(pendDel, lines...)
		case dmp.DiffInsert:
			pendIns = append(pendIns, lines...)
		}
	}
	flush()
	return rows
}

// langFor maps a file extension to a highlight.js language id.
func langFor(path string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "go":
		return "go"
	case "js", "mjs", "cjs":
		return "javascript"
	case "ts":
		return "typescript"
	case "jsx", "tsx":
		return "javascript"
	case "py":
		return "python"
	case "rs":
		return "rust"
	case "c", "h":
		return "c"
	case "cc", "cpp", "cxx", "hpp":
		return "cpp"
	case "java":
		return "java"
	case "rb":
		return "ruby"
	case "php":
		return "php"
	case "sh", "bash", "zsh":
		return "bash"
	case "html", "htm":
		return "xml"
	case "xml", "svg":
		return "xml"
	case "css":
		return "css"
	case "scss", "sass":
		return "scss"
	case "json":
		return "json"
	case "yaml", "yml":
		return "yaml"
	case "toml":
		return "ini"
	case "md", "markdown":
		return "markdown"
	case "sql":
		return "sql"
	case "lua":
		return "lua"
	case "make", "mk", "makefile":
		return "makefile"
	case "dockerfile":
		return "dockerfile"
	}
	if strings.EqualFold(filepath.Base(path), "Makefile") {
		return "makefile"
	}
	if strings.EqualFold(filepath.Base(path), "Dockerfile") {
		return "dockerfile"
	}
	return "plaintext"
}

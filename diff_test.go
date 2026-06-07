package main

import (
	"strings"
	"testing"
)

func joinSpans(spans []Span, skipOp int) string {
	var b strings.Builder
	for _, s := range spans {
		if s.Op == skipOp {
			continue
		}
		b.WriteString(s.Text)
	}
	return b.String()
}

func TestIntraSpans(t *testing.T) {
	left, right := "old value here", "new value here"
	ls, rs := intraSpans(left, right)

	// left spans (equal+delete) must reconstruct the left input
	if got := joinSpans(ls, 1); got != left {
		t.Errorf("left reconstruct = %q want %q", got, left)
	}
	// right spans (equal+insert) must reconstruct the right input
	if got := joinSpans(rs, -1); got != right {
		t.Errorf("right reconstruct = %q want %q", got, right)
	}

	hasDel, hasIns, sharedL := false, false, ""
	for _, s := range ls {
		if s.Op == -1 {
			hasDel = true
		}
		if s.Op == 0 {
			sharedL += s.Text
		}
	}
	for _, s := range rs {
		if s.Op == 1 {
			hasIns = true
		}
	}
	if !hasDel || !hasIns {
		t.Errorf("expected a delete on left (%v) and insert on right (%v)", hasDel, hasIns)
	}
	if !strings.Contains(sharedL, "value here") {
		t.Errorf("equal portion should include the unchanged suffix, got %q", sharedL)
	}
}

func TestIntraSpansIdentical(t *testing.T) {
	ls, rs := intraSpans("same", "same")
	for _, s := range ls {
		if s.Op == -1 {
			t.Error("identical lines should have no delete span")
		}
	}
	for _, s := range rs {
		if s.Op == 1 {
			t.Error("identical lines should have no insert span")
		}
	}
}

func TestBuildDiffRows(t *testing.T) {
	a := "one\nshared\nold value here\nlast\n"
	b := "one\nshared\nnew value here\nlast\nextra\n"
	rows := buildDiffRows(a, b)

	var types []string
	for _, r := range rows {
		types = append(types, r.Type)
	}
	want := []string{"equal", "equal", "change", "equal", "insert"}
	if strings.Join(types, ",") != strings.Join(want, ",") {
		t.Fatalf("row types = %v want %v", types, want)
	}

	// the change row carries intra-line spans on both sides
	chg := rows[2]
	if chg.Left == nil || chg.Right == nil {
		t.Fatal("change row missing a side")
	}
	if chg.Left.N != 3 || chg.Right.N != 3 {
		t.Errorf("change line numbers = L%d R%d want 3/3", chg.Left.N, chg.Right.N)
	}

	// the insert row has only a right side, numbered 5
	ins := rows[4]
	if ins.Left != nil || ins.Right == nil || ins.Right.N != 5 {
		t.Errorf("insert row malformed: %+v", ins)
	}
}

func TestBuildDiffRowsIdentical(t *testing.T) {
	rows := buildDiffRows("a\nb\nc\n", "a\nb\nc\n")
	for _, r := range rows {
		if r.Type != "equal" {
			t.Errorf("identical files should yield only equal rows, got %s", r.Type)
		}
	}
	if len(rows) != 3 {
		t.Errorf("want 3 rows, got %d", len(rows))
	}
}

func TestLangFor(t *testing.T) {
	cases := map[string]string{
		"main.go":       "go",
		"app.py":        "python",
		"index.tsx":     "javascript",
		"server.ts":     "typescript",
		"data.json":     "json",
		"style.css":     "css",
		"Makefile":      "makefile",
		"Dockerfile":    "dockerfile",
		"notes.md":      "markdown",
		"mystery.xyz":   "plaintext",
		"/dev/fd/63":    "plaintext",
		"deep/dir/x.rs": "rust",
	}
	for path, want := range cases {
		if got := langFor(path); got != want {
			t.Errorf("langFor(%q) = %q want %q", path, got, want)
		}
	}
}

func TestSplitLines(t *testing.T) {
	if got := splitLines(""); got != nil {
		t.Errorf("empty = %v want nil", got)
	}
	got := splitLines("a\nb\n")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("a\\nb\\n = %v", got)
	}
	// no trailing newline keeps the last line
	got = splitLines("a\nb")
	if len(got) != 2 || got[1] != "b" {
		t.Errorf("a\\nb = %v", got)
	}
}

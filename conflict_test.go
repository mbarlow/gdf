package main

import "testing"

const twoWay = `package main

func f() {
<<<<<<< HEAD
	return 1
=======
	return 2
>>>>>>> feature
}
`

const diff3 = `a
<<<<<<< ours
x
||||||| base
o
=======
y
>>>>>>> theirs
b
`

func TestParseConflicts2Way(t *testing.T) {
	segs, trail, err := parseConflicts(twoWay)
	if err != nil {
		t.Fatal(err)
	}
	if !trail {
		t.Error("expected trailing newline")
	}
	// text, conflict, text
	if len(segs) != 3 {
		t.Fatalf("want 3 segments, got %d", len(segs))
	}
	c := segs[1]
	if !c.isConflict {
		t.Fatal("segment 1 should be a conflict")
	}
	if c.oursLabel != "HEAD" || c.theirsLabel != "feature" {
		t.Errorf("labels: ours=%q theirs=%q", c.oursLabel, c.theirsLabel)
	}
	if len(c.ours) != 1 || c.ours[0] != "\treturn 1" {
		t.Errorf("ours = %q", c.ours)
	}
	if len(c.theirs) != 1 || c.theirs[0] != "\treturn 2" {
		t.Errorf("theirs = %q", c.theirs)
	}
	if c.hasBase {
		t.Error("2-way conflict should not report base")
	}
}

func TestParseConflictsDiff3(t *testing.T) {
	segs, _, err := parseConflicts(diff3)
	if err != nil {
		t.Fatal(err)
	}
	c := segs[1]
	if !c.hasBase {
		t.Fatal("diff3 should report base")
	}
	if len(c.base) != 1 || c.base[0] != "o" {
		t.Errorf("base = %q", c.base)
	}
	if c.ours[0] != "x" || c.theirs[0] != "y" {
		t.Errorf("ours=%q theirs=%q", c.ours, c.theirs)
	}
	if c.oursLabel != "ours" || c.theirsLabel != "theirs" {
		t.Errorf("labels ours=%q theirs=%q", c.oursLabel, c.theirsLabel)
	}
}

func TestParseConflictsMalformed(t *testing.T) {
	bad := "x\n<<<<<<< HEAD\na\n>>>>>>> feature\n" // no =======
	if _, _, err := parseConflicts(bad); err == nil {
		t.Fatal("expected error on missing =======")
	}
}

func TestReassembleChoices(t *testing.T) {
	segs, trail, err := parseConflicts(twoWay)
	if err != nil {
		t.Fatal(err)
	}
	s := &Session{segments: segs, trailNL: trail}

	cases := map[string]string{
		"ours":     "package main\n\nfunc f() {\n\treturn 1\n}\n",
		"theirs":   "package main\n\nfunc f() {\n\treturn 2\n}\n",
		"both":     "package main\n\nfunc f() {\n\treturn 1\n\treturn 2\n}\n",
		"both-rev": "package main\n\nfunc f() {\n\treturn 2\n\treturn 1\n}\n",
		"none":     "package main\n\nfunc f() {\n}\n",
	}
	for choice, want := range cases {
		got, err := s.reassemble(map[int]string{1: choice})
		if err != nil {
			t.Errorf("%s: %v", choice, err)
			continue
		}
		if got != want {
			t.Errorf("%s:\n got %q\nwant %q", choice, got, want)
		}
	}
}

func TestReassembleBaseDiff3(t *testing.T) {
	segs, trail, err := parseConflicts(diff3)
	if err != nil {
		t.Fatal(err)
	}
	s := &Session{segments: segs, trailNL: trail}
	got, err := s.reassemble(map[int]string{1: "base"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "a\no\nb\n"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestReassembleDefaultsToOurs(t *testing.T) {
	segs, trail, _ := parseConflicts(twoWay)
	s := &Session{segments: segs, trailNL: trail}
	got, _ := s.reassemble(map[int]string{}) // no choice for conflict 1
	want, _ := s.reassemble(map[int]string{1: "ours"})
	if got != want {
		t.Errorf("empty choice should default to ours:\n got %q\nwant %q", got, want)
	}
}

func TestReassembleUnknownChoice(t *testing.T) {
	segs, trail, _ := parseConflicts(twoWay)
	s := &Session{segments: segs, trailNL: trail}
	if _, err := s.reassemble(map[int]string{1: "bogus"}); err == nil {
		t.Fatal("expected error on unknown choice")
	}
}

func TestReassembleNoTrailingNewline(t *testing.T) {
	src := "a\n<<<<<<< HEAD\nx\n=======\ny\n>>>>>>> b" // no final newline
	segs, trail, err := parseConflicts(src)
	if err != nil {
		t.Fatal(err)
	}
	if trail {
		t.Error("should not report trailing newline")
	}
	s := &Session{segments: segs, trailNL: trail}
	got, _ := s.reassemble(map[int]string{1: "ours"})
	if want := "a\nx"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

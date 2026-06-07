package main

import (
	"os"
	"os/exec"
	"testing"
)

func mustGit(t *testing.T, args ...string) {
	t.Helper()
	if _, err := git(args...); err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
}

func writeFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildGitSession(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	t.Chdir(t.TempDir())
	mustGit(t, "init")
	mustGit(t, "config", "user.email", "t@t")
	mustGit(t, "config", "user.name", "t")
	mustGit(t, "config", "commit.gpgsign", "false")

	writeFile(t, "a.txt", "one\ntwo\nthree\n")
	writeFile(t, "del.txt", "gone\n")
	mustGit(t, "add", "-A")
	mustGit(t, "commit", "-m", "init")

	// working-tree changes: modify, delete, add (untracked)
	writeFile(t, "a.txt", "one\nTWO\nthree\nfour\n")
	writeFile(t, "new.txt", "fresh\n")
	if err := os.Remove("del.txt"); err != nil {
		t.Fatal(err)
	}

	sess, err := buildGitSession("", false)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Mode != "git" {
		t.Errorf("mode = %q", sess.Mode)
	}
	if sess.Left.Label != "HEAD" || sess.Right.Label != "working tree" {
		t.Errorf("labels: %q / %q", sess.Left.Label, sess.Right.Label)
	}

	byPath := map[string]FileDiff{}
	for _, f := range sess.Files {
		byPath[f.Path] = f
	}
	if f, ok := byPath["a.txt"]; !ok || f.Status != "M" {
		t.Errorf("a.txt: status=%q ok=%v", f.Status, ok)
	} else if f.Added != 2 || f.Removed != 1 { // TWO change (+1/-1) + four insert (+1)
		t.Errorf("a.txt counts: +%d -%d", f.Added, f.Removed)
	}
	if f, ok := byPath["del.txt"]; !ok || f.Status != "D" {
		t.Errorf("del.txt: status=%q ok=%v", f.Status, ok)
	}
	// untracked file shows with status U, rendered as all-insert
	if f, ok := byPath["new.txt"]; !ok || f.Status != "U" {
		t.Errorf("new.txt: status=%q ok=%v (want U)", f.Status, ok)
	} else if f.Added != 1 || f.Removed != 0 {
		t.Errorf("new.txt counts: +%d -%d (want +1 -0)", f.Added, f.Removed)
	}
	// files are sorted by path
	for i := 1; i < len(sess.Files); i++ {
		if sess.Files[i-1].Path > sess.Files[i].Path {
			t.Errorf("files not sorted: %q before %q", sess.Files[i-1].Path, sess.Files[i].Path)
		}
	}
}

func TestBuildGitSessionNoChanges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	t.Chdir(t.TempDir())
	mustGit(t, "init")
	mustGit(t, "config", "user.email", "t@t")
	mustGit(t, "config", "user.name", "t")
	mustGit(t, "config", "commit.gpgsign", "false")
	writeFile(t, "a.txt", "x\n")
	mustGit(t, "add", "-A")
	mustGit(t, "commit", "-m", "init")

	if _, err := buildGitSession("", false); err == nil {
		t.Fatal("expected error when there are no changes")
	}
}

func TestBuildGitSessionNotARepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	t.Chdir(t.TempDir())
	if _, err := buildGitSession("", false); err == nil {
		t.Fatal("expected error outside a git repo")
	}
}

func TestCountChanges(t *testing.T) {
	rows := buildDiffRows("a\nb\nc\n", "a\nB\nc\nd\n") // b->B (change), d (insert)
	add, rem := countChanges(rows)
	if add != 2 || rem != 1 {
		t.Errorf("add=%d rem=%d, want 2/1", add, rem)
	}
}

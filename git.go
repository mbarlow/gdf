package main

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// git runs a git command in the current directory and returns stdout.
func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return string(out), nil
}

// gitBlob returns the content of <rev>:<path>, or "" if it doesn't exist
// (e.g. an added file has no HEAD blob). rev "" with ":path" reads the index.
func gitBlob(spec string) string {
	out, err := git("show", spec)
	if err != nil {
		return ""
	}
	return out
}

// countChanges tallies added/removed lines from rendered rows.
func countChanges(rows []Row) (added, removed int) {
	for _, r := range rows {
		switch r.Type {
		case "insert":
			added++
		case "delete":
			removed++
		case "change":
			added++
			removed++
		}
	}
	return
}

// buildGitSession diffs the working tree (or index) against a base ref and
// returns a multi-file session. rev selects the base ("" = HEAD); staged
// compares the index instead of the working tree.
func buildGitSession(rev string, staged bool) (*Session, error) {
	if _, err := git("rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, fmt.Errorf("not a git repository")
	}

	base := rev
	if base == "" {
		base = "HEAD"
	}

	var nameStatus string
	var err error
	rightLabel := "working tree"
	if staged {
		nameStatus, err = git("diff", "--cached", "--name-status", "-M", base)
		rightLabel = "index"
	} else {
		nameStatus, err = git("diff", "--name-status", "-M", base)
	}
	if err != nil {
		return nil, err
	}

	var files []FileDiff
	for _, line := range strings.Split(strings.TrimRight(nameStatus, "\n"), "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		status := f[0]
		letter := status[:1]
		oldPath, newPath := "", ""
		switch {
		case (letter == "R" || letter == "C") && len(f) >= 3:
			oldPath, newPath = f[1], f[2]
		case len(f) >= 2:
			oldPath, newPath = f[1], f[1]
		default:
			continue
		}

		// base (old) content
		oldContent := ""
		if letter != "A" {
			oldContent = gitBlob(base + ":" + oldPath)
		}
		// new content: index blob when staged, else the working file
		newContent := ""
		if letter != "D" {
			if staged {
				newContent = gitBlob(":" + newPath)
			} else {
				newContent, _ = readSide(newPath)
			}
		}

		rows := buildDiffRows(oldContent, newContent)
		add, rem := countChanges(rows)
		path := newPath
		if letter == "D" {
			path = oldPath
		}
		files = append(files, FileDiff{
			Path:     path,
			Status:   letter,
			Language: langFor(path),
			Added:    add,
			Removed:  rem,
			Rows:     rows,
		})
	}

	// Untracked files: not in HEAD or the index, but present and not ignored.
	// Only relevant to the working tree (the index can't contain untracked).
	if !staged {
		untracked, _ := git("ls-files", "--others", "--exclude-standard")
		for _, path := range strings.Split(strings.TrimRight(untracked, "\n"), "\n") {
			if path == "" {
				continue
			}
			content, _ := readSide(path)
			rows := buildDiffRows("", content)
			add, rem := countChanges(rows)
			files = append(files, FileDiff{
				Path:     path,
				Status:   "U", // untracked
				Language: langFor(path),
				Added:    add,
				Removed:  rem,
				Rows:     rows,
			})
		}
	}

	if len(files) == 0 {
		where := "working tree"
		if staged {
			where = "index"
		}
		return nil, fmt.Errorf("no changes in %s vs %s", where, base)
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	return &Session{
		Mode:     "git",
		Filename: fmt.Sprintf("%d changed file(s)", len(files)),
		Left:     Pane{Label: base, Sub: "base"},
		Right:    Pane{Label: rightLabel, Sub: "current"},
		Files:    files,
	}, nil
}

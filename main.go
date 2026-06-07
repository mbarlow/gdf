// Command gdf is a meld-flavored side-by-side diff and merge tool that renders
// in a Chrome app window. In merge mode it operates on a git-conflicted file,
// lets you pick per-conflict resolutions, and writes the result back so it can
// be wired up as `git mergetool`.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// version is overwritten at build time via -ldflags "-X main.version=...".
var version = "dev"

func usage() {
	fmt.Fprint(os.Stderr, `gdf — side-by-side diff / merge in a Chrome app window

usage:
  gdf merge <conflicted-file>     resolve git conflict markers, write result back
  gdf diff  <fileA> <fileB>       side-by-side diff of two files
  gdf <fileA> <fileB>             alias for diff

flags:
  --theme   light|dark|auto   (default auto)
  --port    fixed port        (default random)
  --no-open print URL instead of launching Chrome
  --lang    force syntax language (for paths without a usable extension)
  --version print version and exit

git mergetool:
  git config --global mergetool.gdf.cmd 'gdf merge "$MERGED"'
  git config --global mergetool.gdf.trustExitCode true
  git mergetool -t gdf
`)
}

func main() {
	fs := flag.NewFlagSet("gdf", flag.ExitOnError)
	theme := fs.String("theme", "auto", "light|dark|auto")
	port := fs.Int("port", 0, "fixed port (0 = random)")
	noOpen := fs.Bool("no-open", false, "print URL instead of launching Chrome")
	lang := fs.String("lang", "", "force syntax language (e.g. go, python) when the path has no usable extension")
	showVer := fs.Bool("version", false, "print version and exit")
	fs.Usage = usage

	// Parse flags from anywhere on the line (before, after, or between the
	// subcommand and its file arguments) by parsing-then-collecting positionals.
	var args []string
	rest := os.Args[1:]
	for len(rest) > 0 {
		if err := fs.Parse(rest); err != nil {
			os.Exit(2)
		}
		more := fs.Args()
		if len(more) == 0 {
			break
		}
		args = append(args, more[0])
		rest = more[1:]
	}
	if *showVer {
		fmt.Printf("gdf %s\n", version)
		return
	}
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	var sess *Session
	var err error

	switch args[0] {
	case "merge":
		if len(args) != 2 {
			usage()
			os.Exit(2)
		}
		sess, err = buildMergeSession(args[1])
	case "diff":
		if len(args) != 3 {
			usage()
			os.Exit(2)
		}
		sess, err = buildDiffSession(args[1], args[2])
	default:
		// bare two-file form: gdf a b
		if len(args) != 2 {
			usage()
			os.Exit(2)
		}
		sess, err = buildDiffSession(args[0], args[1])
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "gdf: %v\n", err)
		os.Exit(1)
	}

	sess.Theme = *theme
	if *lang != "" {
		sess.Language = *lang
	}
	if sess.Filename == "" {
		sess.Filename = "(diff)"
	} else {
		sess.Filename = filepath.Clean(sess.Filename)
	}

	code := serve(sess, *port, *noOpen)
	os.Exit(code)
}

# gdf

A meld-flavored side-by-side diff and merge tool that renders in a Chrome app
window. Vanilla JS/CSS/SVG, no build step. The Go binary is the long-running
process: it serves the UI, and when you resolve a merge it writes the result
back and exits — so it drops straight into `git mergetool`.

## What it does

- **Merge mode** — reads a git-conflicted file, parses `<<<<<<< ======= >>>>>>>`
  markers (2-way and diff3), shows ours/theirs side by side. Pick per conflict:
  **ours / theirs / both↓ / both↑ / base / skip**, hit **Merge & Apply**. The
  binary reassembles the file, writes it, exits 0.
- **Diff mode** — side-by-side diff of two files.
- Branch + filename labels in the headers, dual line-number gutters.
- Colorized diff with **intra-line** char highlighting (the changed substring
  inside a line, meld-style) via Myers diff.
- Syntax highlighting (highlight.js), light / dark theme (auto-detects, toggle).
- Fonts: Inter (UI), JetBrains Mono / SUSE Mono (code).

## Install

```bash
make install          # -> ~/.local/bin/gdf
```

Requires Go 1.25+ and a Chromium-family browser on PATH (google-chrome,
chromium, brave, edge).

## Usage

```bash
gdf merge <conflicted-file>     # resolve conflict markers, write back
gdf diff  <fileA> <fileB>       # side-by-side diff
gdf <fileA> <fileB>             # alias for diff

# flags (anywhere on the line):
#   --theme light|dark|auto    default auto
#   --port  N                  fixed port (default random)
#   --no-open                  print URL instead of launching Chrome
```

### As a git mergetool

```bash
make git-config
# or manually:
git config --global mergetool.gdf.cmd 'gdf merge "$MERGED"'
git config --global mergetool.gdf.trustExitCode true

git mergetool -t gdf
```

Exit codes: `0` = merged/applied (or diff closed), `1` = aborted (Abort button,
window closed, or Esc). With `trustExitCode true`, git only marks a path
resolved when gdf exits 0.

### Keys

- `Ctrl/Cmd+Enter` — Merge & Apply (when all conflicts resolved)
- `Esc` — Abort

## How it works

```
git mergetool ──► gdf merge $MERGED ──► HTTP server on 127.0.0.1:rand
                                          │
                          Chrome --app ◄──┘  (frameless app window)
                                          │
   you pick choices ──► POST /api/resolve ─► reassemble ─► write $MERGED ─► exit 0
```

The conflicted file is the source of truth: gdf splits it into literal segments
and conflict regions, renders the regions, and on resolve stitches the literal
text back together with your chosen blocks. No reliance on separate
LOCAL/BASE/REMOTE temp files — branch labels come from the markers themselves.

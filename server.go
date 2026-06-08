package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"
)

//go:embed web
var webFS embed.FS

type resolveReq struct {
	Choices map[int]string `json:"choices"`
}

// serve starts the local server, opens the UI, and blocks until the user
// resolves, cancels, or closes the window. Returns the process exit code.
func serve(sess *Session, port int, noOpen bool) int {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "gdf: listen: %v\n", err)
		return 1
	}
	url := fmt.Sprintf("http://%s/", ln.Addr().String())

	result := make(chan int, 1)
	static, _ := fs.Sub(webFS, "web")

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(static)))

	mux.HandleFunc("/api/session", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sess)
	})

	mux.HandleFunc("/api/resolve", func(w http.ResponseWriter, r *http.Request) {
		if sess.Mode != "merge" {
			http.Error(w, "not a merge session", http.StatusBadRequest)
			return
		}
		var req resolveReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		merged, err := sess.reassemble(req.Choices)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := os.WriteFile(sess.mergePath, []byte(merged), 0o644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
		go finish(result, 0)
	})

	mux.HandleFunc("/api/cancel", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
		go finish(result, 1)
	})

	mux.HandleFunc("/api/close", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
		go finish(result, 0)
	})

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()

	var chrome *exec.Cmd
	if noOpen {
		fmt.Printf("gdf serving at %s\n", url)
	} else {
		chrome = launchChrome(url)
		if chrome == nil {
			fmt.Printf("gdf: no Chrome found; open manually: %s\n", url)
		} else {
			// window closed without an explicit action -> treat as cancel(merge)/ok(diff)
			go func() {
				_ = chrome.Wait()
				if sess.Mode == "merge" {
					finish(result, 1)
				} else {
					finish(result, 0)
				}
			}()
		}
	}

	code := <-result

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = srv.Shutdown(ctx)
	if chrome != nil && chrome.Process != nil {
		_ = chrome.Process.Kill()
	}
	cleanupProfile()
	return code
}

var finished = make(chan struct{})

// finish delivers the first exit code and ignores subsequent ones.
func finish(result chan int, code int) {
	select {
	case <-finished:
	default:
		close(finished)
		result <- code
	}
}

var profileDir string

func cleanupProfile() {
	if profileDir != "" {
		_ = os.RemoveAll(profileDir)
	}
}

// launchChrome starts Chrome/Chromium in app mode against url.
func launchChrome(url string) *exec.Cmd {
	candidates := []string{
		"google-chrome-stable", "google-chrome", "chromium", "chromium-browser",
		"brave", "brave-browser", "microsoft-edge",
	}
	var bin string
	for _, c := range candidates {
		if p, err := exec.LookPath(c); err == nil {
			bin = p
			break
		}
	}
	if bin == "" {
		// PATH lookup misses on macOS (Chrome lives in an .app bundle) and
		// on some Linux installs. Fall back to known absolute locations.
		for _, p := range chromeFallbackPaths() {
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				bin = p
				break
			}
		}
	}
	if bin == "" {
		return nil
	}
	dir, err := os.MkdirTemp("", "gdf-profile-")
	if err == nil {
		profileDir = dir
	}
	args := []string{
		"--app=" + url,
		"--new-window",
		"--no-first-run",
		"--no-default-browser-check",
		"--window-size=1400,900",
	}
	if profileDir != "" {
		args = append(args, "--user-data-dir="+profileDir)
	}
	cmd := exec.Command(bin, args...)
	if err := cmd.Start(); err != nil {
		return nil
	}
	return cmd
}

// chromeFallbackPaths returns absolute Chrome/Chromium locations that aren't
// usually on PATH, keyed by OS.
func chromeFallbackPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	default:
		return []string{
			"/usr/bin/google-chrome-stable",
			"/usr/bin/google-chrome",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
		}
	}
}

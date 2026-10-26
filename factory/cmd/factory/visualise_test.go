package main

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lean-software-production/my-factory-reference/factory/internal/replayweb"
)

// said runs the command with the arguments and is its exit status, and
// what it wrote to the standard output and the standard error.
func said(t *testing.T, args ...string) (status int, stdout, stderr string) {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()
	outC, errC := make(chan string), make(chan string)
	for r, c := range map[*os.File]chan string{outR: outC, errR: errC} {
		go func() {
			b, _ := io.ReadAll(r)
			c <- string(b)
		}()
	}
	status = command(args)
	outW.Close()
	errW.Close()
	return status, <-outC, <-errC
}

func TestVisualiseRunIsHidden(t *testing.T) {
	status, stdout, stderr := said(t, "--help")
	if status != 0 {
		t.Errorf("--help exited %d", status)
	}
	if strings.Contains(stdout+stderr, "visualise") {
		t.Errorf("--help mentions visualise:\n%s%s", stdout, stderr)
	}
	main, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	comment, _, _ := strings.Cut(string(main), "\npackage main")
	if strings.Contains(comment, "visualise") {
		t.Error("main.go's package comment mentions visualise")
	}
}

func TestVisualiseRunNeedsOneRecord(t *testing.T) {
	for _, args := range [][]string{
		{"visualise-run"},
		{"visualise-run", "a.jsonl", "b.jsonl"},
		{"visualise-run", "--port", "0"},
		{"visualise-run", "--nope", "a.jsonl"},
	} {
		if status, _, _ := said(t, args...); status != 2 {
			t.Errorf("factory %s exited %d, not 2", strings.Join(args, " "), status)
		}
	}
}

func TestVisualiseRunSaysWhenTheRecordIsMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nowhere", "record.jsonl")
	status, _, stderr := said(t, "visualise-run", missing)
	if status != 1 {
		t.Errorf("exited %d, not 1", status)
	}
	if !strings.Contains(stderr, missing) {
		t.Errorf("the error does not name %s: %q", missing, stderr)
	}
}

func TestReadRecordIgnoresAPartialLastLine(t *testing.T) {
	dir := t.TempDir()
	whole := `{"time":"2026-01-02T03:04:05Z","kind":"run","run":"r","state":"started"}` + "\n" +
		`{"time":"2026-01-02T03:04:06Z","kind":"run","run":"r","state":"finished"}` + "\n"
	path := filepath.Join(dir, "record.jsonl")
	if err := os.WriteFile(path, []byte(whole+`{"time":"2026-`), 0o644); err != nil {
		t.Fatal(err)
	}
	events, err := readRecord(path)
	if err != nil || len(events) != 2 {
		t.Errorf("read %d events, %v; want 2 and no error", len(events), err)
	}

	bad := filepath.Join(dir, "bad.jsonl")
	lines := strings.SplitAfter(whole, "\n")
	if err := os.WriteFile(bad, []byte(lines[0]+"not json\n"+lines[1]), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readRecord(bad); err == nil || !strings.Contains(err.Error(), bad) {
		t.Errorf("a bad line in the middle gave %v, want an error naming %s", err, bad)
	}
}

func TestNamesComeFromThePathWithoutRunJSON(t *testing.T) {
	demo := runNamesFor(demoRecord)
	if demo.Run != "tetris-007" || demo.Line != "implement-with-3-big-brains" ||
		!strings.HasSuffix(filepath.ToSlash(demo.Target), "demo/implement-with-3-big-brains") {
		t.Errorf("the demo's names are %+v", demo)
	}
	ride := runNamesFor("../../../tetris/release-ride-3/.assembly-lines/ralph-loop/runs/ride/record.jsonl")
	if ride.Run != "ride" || ride.Line != "ralph-loop" {
		t.Errorf("the ride's names are %+v", ride)
	}
}

func TestNamesPreferTheLineRunJSONSays(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "t", linesFolder, "folder-name", "runs", "r1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, settingsFile), []byte(`{"line":"said-in-run-json","seed":"s"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := runNamesFor(filepath.Join(dir, recordFile)); got.Line != "said-in-run-json" || got.Run != "r1" {
		t.Errorf("names are %+v", got)
	}
}

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	return replayHandler([]byte(`{"run":"r"}`), replayweb.Files())
}

func get(h http.Handler, path string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	for i := 0; i+1 < len(headers); i += 2 {
		if headers[i] == "Host" {
			req.Host = headers[i+1]
		} else {
			req.Header.Set(headers[i], headers[i+1])
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHandlerServesThePageAndTheRun(t *testing.T) {
	h := testHandler(t)
	for _, c := range []struct {
		path, kind string
		status     int
	}{
		{"/", "text/html", 200},
		{"/run.json", "application/json", 200},
		{"/kit/sketchbook.css", "text/css", 200},
		{"/kit/sketchbook.js", "text/javascript", 200},
		{"/kit/fonts/patrick-hand.ttf", "font/ttf", 200},
		{"/nope", "", 404},
		{"/kit", "", 404},
		{"/kit/", "", 404},
	} {
		rec := get(h, c.path)
		if rec.Code != c.status {
			t.Errorf("GET %s: %d, not %d", c.path, rec.Code, c.status)
		}
		if got := rec.Header().Get("Content-Type"); c.kind != "" && !strings.HasPrefix(got, c.kind) {
			t.Errorf("GET %s: Content-Type %q, not %s", c.path, got, c.kind)
		}
	}
	if body := get(h, "/run.json").Body.String(); body != `{"run":"r"}` {
		t.Errorf("GET /run.json: %q", body)
	}
	if body := get(h, "/").Body.String(); !strings.Contains(body, "replay.js") {
		t.Errorf("GET /: %q", body)
	}
	head := httptest.NewRecorder()
	h.ServeHTTP(head, httptest.NewRequest("HEAD", "/", nil))
	if head.Code != 200 || head.Body.Len() != 0 {
		t.Errorf("HEAD /: %d with %d bytes", head.Code, head.Body.Len())
	}
}

func TestNothingBlocksFraming(t *testing.T) {
	h := testHandler(t)
	for _, path := range []string{"/", "/run.json", "/kit/sketchbook.css", "/kit/fonts/patrick-hand.ttf", "/nope"} {
		header := get(h, path).Header()
		for _, name := range []string{"X-Frame-Options", "Content-Security-Policy", "Content-Security-Policy-Report-Only"} {
			if v := header.Get(name); v != "" {
				t.Errorf("GET %s: %s is %q", path, name, v)
			}
		}
		if got := header.Get("Cache-Control"); got != "no-store" {
			t.Errorf("GET %s: Cache-Control is %q", path, got)
		}
	}
}

func TestAnyHostIsAnswered(t *testing.T) {
	rec := get(testHandler(t), "/", "Host", "fluffy-guide-8080.app.github.dev", "Origin", "https://github.dev")
	if rec.Code != 200 {
		t.Errorf("GET / from a forwarded port: %d", rec.Code)
	}
}

func TestTheEmbeddedFilesUseNoNetwork(t *testing.T) {
	files := replayweb.Files()
	seen := 0
	err := fs.WalkDir(files, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		switch filepath.Ext(name) {
		case ".html", ".css", ".js":
		default:
			return nil
		}
		seen++
		raw, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		text := strings.ReplaceAll(string(raw), "http://www.w3.org/", "")
		for _, banned := range []string{"http://", "https://", "http-equiv", "@import"} {
			if strings.Contains(text, banned) {
				t.Errorf("%s contains %q", name, banned)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen < 3 {
		t.Errorf("looked at only %d files", seen)
	}
}

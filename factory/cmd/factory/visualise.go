package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lean-software-production/my-factory-reference/factory/internal/replayweb"
	"github.com/lean-software-production/my-factory-reference/factory/internal/watch"
)

// visualiseCommand replays a finished run in a browser: it reads the run's
// record and its assembly line, works out the replay, and serves it, and the
// page that draws it, from this computer until it is interrupted. It is not
// in the factory's usage: it is shown to students by whoever teaches them.
//
// The page is served on 127.0.0.1 only, and answers whatever name it is asked
// for, so that a port a code editor forwards can reach it. It needs nothing
// from the network, and nothing it sends stops a page from showing it inside
// another.
func visualiseCommand(args []string) error {
	flags := flag.NewFlagSet("factory visualise-run", flag.ContinueOnError)
	graph := flags.String("graph", "", "the assembly line's graph (default: assembly-line.dot, two folders up from the record's folder)")
	port := flags.Int("port", 0, "the port to serve on (default: any that is free)")
	var paths []string
	for len(args) > 0 {
		if err := flags.Parse(args); err != nil {
			return usage(err)
		}
		if parsed := args[:len(args)-flags.NArg()]; len(parsed) > 0 && parsed[len(parsed)-1] == "--" {
			paths = append(paths, flags.Args()...)
			break
		}
		if flags.NArg() > 0 {
			paths = append(paths, flags.Arg(0))
		}
		args = flags.Args()
		if len(args) > 0 {
			args = args[1:]
		}
	}
	if len(paths) != 1 {
		return usage(errors.New("visualise-run names one record: factory visualise-run [--graph <file>] [--port <n>] <record.jsonl>"))
	}
	if *port < 0 || *port > 65535 {
		return usage(fmt.Errorf("%d is not a port", *port))
	}
	recordPath := paths[0]
	graphFlag := *graph
	if *graph == "" {
		*graph = filepath.Join(filepath.Dir(filepath.Dir(absolute(filepath.Dir(recordPath)))), "assembly-line.dot")
	}
	events, err := readRecord(recordPath)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(*graph)
	if err != nil {
		return fmt.Errorf("could not read the assembly line: %v%s", err, graphHint(graphFlag))
	}
	line, err := parseAssemblyLine(raw)
	if err != nil {
		return fmt.Errorf("could not read the assembly line %s: %v", *graph, err)
	}
	names := runNamesFor(recordPath, graphFlag, events)
	doc, err := json.Marshal(buildReplay(line, events, names))
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(*port)))
	if err != nil {
		return fmt.Errorf("could not serve the replay: %v", err)
	}
	ctx, stop := stoppedBySignals()
	defer stop()
	where := names.Line
	if names.Target != "" {
		where += " · " + names.Target
	}
	fmt.Printf("Replaying run %q on %s\n", names.Run, where)
	fmt.Printf("Open http://localhost:%d/\n", listener.Addr().(*net.TCPAddr).Port)
	fmt.Println(`In VS Code, choose "Preview in Editor" when it offers the port, or run "Simple Browser: Show" and paste the address.`)
	fmt.Println("Ctrl-C stops it.")
	return serveUntil(ctx, listener, replayHandler(doc, replayweb.Files()))
}

// graphHint is what to add to the error of a graph that could not be read:
// when it was not named, that it can be.
func graphHint(graphFlag string) string {
	if graphFlag != "" {
		return ""
	}
	return "; name it with --graph <file>"
}

// serveUntil serves on the listener until ctx is done, then stops serving,
// giving the requests being answered a moment to finish.
func serveUntil(ctx context.Context, listener net.Listener, h http.Handler) error {
	server := &http.Server{Handler: h}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	select {
	case err := <-served:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		server.Close()
	}
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// absolute is the path made absolute, or as it was if the working folder
// cannot be found.
func absolute(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// readRecord is the events of a run's record, in order. A last line the run
// had not finished writing is not an event yet, and is left out; a line
// that is not an event anywhere else is an error.
func readRecord(recordPath string) ([]watch.Event, error) {
	file, err := os.Open(recordPath)
	if err != nil {
		return nil, fmt.Errorf("could not read the run's record: %v", err)
	}
	defer file.Close()
	var events []watch.Event
	lines := bufio.NewReader(file)
	for n := 1; ; n++ {
		text, err := lines.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("could not read the run's record %s: %v", recordPath, err)
		}
		complete := err == nil
		if len(bytes.TrimSpace(text)) > 0 {
			var ev watch.Event
			if jerr := json.Unmarshal(text, &ev); jerr == nil {
				events = append(events, ev)
			} else if complete {
				return nil, fmt.Errorf("the run's record %s is not readable: line %d is not an event: %v", recordPath, n, jerr)
			}
		}
		if !complete {
			return events, nil
		}
	}
}

// runNamesFor is what the page calls a run: the run is named by the first
// run event of the record that names one, else by the folder of the record;
// the line by the run.json beside the record, else by the folder of the
// graph if one was named with --graph (graphFlag), else by the folder two up
// from the record's. The target is only known when the record is kept as
// <target>/.assembly-lines/<line>/runs/<run>/record.jsonl, and is empty
// otherwise.
func runNamesFor(recordPath, graphFlag string, events []watch.Event) replayNames {
	runDir := absolute(filepath.Dir(recordPath))
	lineDir := filepath.Dir(filepath.Dir(runDir))
	names := replayNames{Run: filepath.Base(runDir), Line: filepath.Base(lineDir)}
	if graphFlag != "" {
		names.Line = filepath.Base(filepath.Dir(absolute(graphFlag)))
	}
	if i := slices.IndexFunc(events, func(ev watch.Event) bool { return ev.Kind == watch.Run && ev.Run != "" }); i >= 0 {
		names.Run = events[i].Run
	}
	if raw, err := os.ReadFile(filepath.Join(runDir, settingsFile)); err == nil {
		var s settings
		if json.Unmarshal(raw, &s) == nil && s.Line != "" {
			names.Line = s.Line
		}
	}
	if filepath.Base(filepath.Dir(runDir)) == "runs" && filepath.Base(filepath.Dir(lineDir)) == linesFolder {
		names.Target = shownTarget(filepath.Dir(filepath.Dir(lineDir)))
	}
	return names
}

// contentTypes are what the page's files are, by their endings. They are
// said here and not asked of the computer, which may not know them.
var contentTypes = map[string]string{
	".html": "text/html; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".js":   "text/javascript; charset=utf-8",
	".json": "application/json",
	".svg":  "image/svg+xml",
	".txt":  "text/plain; charset=utf-8",
	".md":   "text/markdown; charset=utf-8",
	".ttf":  "font/ttf",
}

// replayHandler serves the page from files, and the replay, doc, as
// /run.json. It looks at nothing but the path, so that it answers whatever
// address a forwarded port is reached by, and it says nothing that stops a
// page showing it from framing it. What it sends is never kept: the replay
// is built once, and the browser asks for it again each time because of
// Cache-Control: no-store.
func replayHandler(doc []byte, files fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, kind := doc, contentTypes[".json"]
		if name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/"); name != "run.json" {
			if name == "" {
				name = "index.html"
			}
			var err error
			if body, err = readFile(files, name); err != nil {
				http.NotFound(w, r)
				return
			}
			kind = contentTypes[strings.ToLower(path.Ext(name))]
			if kind == "" {
				kind = "application/octet-stream"
			}
		}
		w.Header().Set("Content-Type", kind)
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(body))
	})
}

// readFile is the file of that name, which is not a folder.
func readFile(files fs.FS, name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrNotExist
	}
	return fs.ReadFile(files, name)
}

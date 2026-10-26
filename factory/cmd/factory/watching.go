package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lean-software-production/my-factory-reference/factory/internal/watch"
)

// How often watching looks for more of a run's record, and asks the
// factory whether the run is still running.
const (
	pollEvery   = 20 * time.Millisecond
	statusEvery = 500 * time.Millisecond
)

// interrupted is the exit status of watching that was interrupted, as a
// shell reports a process killed by SIGINT.
const interrupted = 130

// watchCommand shows a run: everything its record holds so far, then what
// is added to it, until the run has ended. It fails unless the run
// finished.
func watchCommand(args []string) error {
	i, err := runFlags("watch", args)
	if err != nil {
		return err
	}
	line, err := findRun(i.target, i.run)
	if err != nil {
		return err
	}
	if line == "" {
		return fmt.Errorf("the %q run has not been started in %s", i.run, i.target)
	}
	f := runFolder{target: i.target, name: i.run, settings: settings{Line: line},
		dir: filepath.Join(i.target, linesFolder, line, "runs", i.run)}
	w := watch.New(os.Stdout, line)
	w.Names(machineNames(f.lineDir())...)
	w.SetBar(watch.Bar{Run: f.name, Target: shownTarget(f.target), Route: routeOf(f.lineDir())})
	// Watching can be interrupted. Either way it ends, the status bar is
	// wiped before anything more is said.
	ctx, stop := stoppedBySignals()
	defer stop()
	defer w.Close()
	last, err := follow(ctx, f, w)
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if errors.Is(err, context.Canceled) {
		return exitStatus(interrupted)
	}
	if err != nil {
		return err
	}
	if !last.Ended() {
		fmt.Printf("run %s is not running, and its record does not say how it ended\n", f.name)
		return exitStatus(1)
	}
	if last.State != watch.Finished {
		return exitStatus(1)
	}
	return nil
}

// follow renders the run's record from its start, then what is added to it,
// until the factory is not running the run, or ctx is done, when it returns
// the context's error. It returns the last event about the run starting or
// ending.
func follow(ctx context.Context, f runFolder, w *watch.Watcher) (watch.Event, error) {
	file, err := os.Open(f.record())
	if errors.Is(err, os.ErrNotExist) {
		return watch.Event{}, fmt.Errorf("the %q run has no record", f.name)
	}
	if err != nil {
		return watch.Event{}, err
	}
	defer file.Close()
	r := bufio.NewReader(file)
	var (
		last    watch.Event
		partial []byte    // a line still being written
		asked   time.Time // when the factory was last asked about the run
		settled bool      // the factory is not running the run: the record is all there will be
	)
	for {
		if err := ctx.Err(); err != nil {
			return last, err
		}
		line, err := r.ReadBytes('\n')
		partial = append(partial, line...)
		if err == nil {
			var ev watch.Event
			if jsonErr := json.Unmarshal(partial, &ev); jsonErr != nil {
				return last, fmt.Errorf("could not read the run's record, %s: %v", f.record(), jsonErr)
			}
			partial = partial[:0]
			if err := w.Render(ev); err != nil {
				return last, err
			}
			if ev.Kind == watch.Run {
				last = ev
			}
			continue
		}
		if err != io.EOF {
			return last, err
		}
		if settled {
			return last, nil
		}
		// At the end of the record so far. A run that has ended may still be
		// committing its folder; one that is not running has written all it
		// will, but read it once more for what it wrote since.
		if last.Ended() || time.Since(asked) > statusEvery {
			asked = time.Now()
			settled = !f.isRunning()
			if settled {
				continue
			}
		}
		if err := w.Tick(time.Now()); err != nil {
			return last, err
		}
		time.Sleep(pollEvery)
	}
}

// isRunning says whether the factory is running the run now.
func (f runFolder) isRunning() bool {
	rep, err := call(request{Op: "status"}, false)
	return err == nil && rep.Run == f.name && rep.Target == f.target
}

// shownTarget is the target as the status bar names it: its path from the
// folder watch is run in, if it is in it, and its whole path otherwise.
func shownTarget(target string) string {
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, target); err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return rel
		}
	}
	return target
}

// machineNames are the names of the machines of the assembly line in the
// folder, or none if they cannot be read.
func machineNames(lineDir string) []string {
	entries, _ := os.ReadDir(filepath.Join(lineDir, "machines"))
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

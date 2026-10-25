package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// A target holds its assembly lines in .assembly-lines/, a folder each,
// named for the line. A run lives in its line's folder, in runs/<name>/:
// its plan, plan.md, whatever its machines keep for it, and run.json, which
// says which line the run executes and on which seed. A run is found by its
// target and its name; nothing of it is kept in the factory.
const (
	linesFolder  = ".assembly-lines"
	settingsFile = "run.json"
	planFile     = "plan.md"
)

// invocation is what the caller asked for: a run of a target.
type invocation struct {
	run, target string
	line, seed  string // given the first time; after that the run remembers them
	attempts    int
}

// settings are what a run remembers: the line it executes and its seed. The
// seed is kept as a path from the target, so the record is the same wherever
// the target is cloned.
type settings struct {
	Line string `json:"line"`
	Seed string `json:"seed"`
}

// A runFolder is a run's folder on its line.
type runFolder struct {
	target, name string
	settings
	dir     string // .assembly-lines/<line>/runs/<name>
	started bool   // whether the run existed before this invocation
}

func (f runFolder) plan() string     { return filepath.Join(f.dir, planFile) }
func (f runFolder) lineDir() string  { return filepath.Join(f.target, linesFolder, f.Line) }
func (f runFolder) seedPath() string { return resolveSeed(f.target, f.Seed) }

func resolveSeed(target, seed string) string {
	if filepath.IsAbs(seed) {
		return seed
	}
	return filepath.Join(target, seed)
}

func (f runFolder) save() error {
	raw, err := json.MarshalIndent(f.settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(f.dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(f.dir, settingsFile), append(raw, '\n'), 0o644)
}

// openRun finds the run the invocation names in its target, and checks what
// it was given against what the run remembers. A run that has not been
// started is given its line and seed here, but nothing is written yet.
func (i invocation) openRun() (runFolder, error) {
	if i.target == "" {
		return runFolder{}, errors.New("a target is required: give one with --target <folder>")
	}
	if i.run == "" || i.run != filepath.Base(i.run) || strings.HasPrefix(i.run, ".") {
		return runFolder{}, errors.New("a run is required: name it with --run <name>")
	}
	target, err := filepath.Abs(i.target)
	if err != nil {
		return runFolder{}, err
	}
	seed := ""
	if i.seed != "" {
		// Relative paths are the caller's: resolve them from the working directory.
		if seed, err = filepath.Abs(i.seed); err != nil {
			return runFolder{}, err
		}
	}

	f := runFolder{target: target, name: i.run}
	found, err := findRun(target, i.run)
	if err != nil {
		return runFolder{}, err
	}
	if found != "" {
		f.started = true
		f.Line = found
		f.dir = filepath.Join(target, linesFolder, found, "runs", i.run)
		if err := f.load(); err != nil {
			return runFolder{}, err
		}
		if i.line != "" && i.line != f.Line {
			return runFolder{}, fmt.Errorf("the %q run already has a line, %q, so it cannot be given %q: a run's line cannot be changed", i.run, f.Line, i.line)
		}
		if seed != "" && seed != f.seedPath() {
			return runFolder{}, fmt.Errorf("the %q run already has a seed, %s, so it cannot be given %s: a run's seed cannot be changed", i.run, f.seedPath(), seed)
		}
	} else {
		if i.line == "" {
			return runFolder{}, fmt.Errorf("the %q run has not been started in %s: start it with --line <line> and --seed <file>", i.run, target)
		}
		if seed == "" {
			return runFolder{}, errors.New("there is no seed: give one with --seed <file>")
		}
		if i.line != filepath.Base(i.line) || strings.HasPrefix(i.line, ".") {
			return runFolder{}, fmt.Errorf("%q is not the name of a line", i.line)
		}
		rel, err := filepath.Rel(target, seed)
		if err != nil {
			rel = seed
		}
		f.settings = settings{i.line, rel}
		f.dir = filepath.Join(target, linesFolder, i.line, "runs", i.run)
	}
	if info, err := os.Stat(f.seedPath()); err != nil || info.IsDir() {
		return runFolder{}, fmt.Errorf("there is no seed at %s", f.seedPath())
	}
	if info, err := os.Stat(f.lineDir()); err != nil || !info.IsDir() {
		return runFolder{}, fmt.Errorf("the target %s has no assembly line called %q (looked for %s)", target, f.Line, f.lineDir())
	}
	return f, nil
}

// findRun is the line a run of that name is on in the target: a run's name is
// unique in its target, whichever line it executes. It is "" if the run has
// not been started.
func findRun(target, name string) (string, error) {
	lines, err := os.ReadDir(filepath.Join(target, linesFolder))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	var on []string
	for _, line := range lines {
		if _, err := os.Stat(filepath.Join(target, linesFolder, line.Name(), "runs", name, settingsFile)); err == nil {
			on = append(on, line.Name())
		}
	}
	if len(on) > 1 {
		return "", fmt.Errorf("the %q run is on more than one line of %s (%s): a run's name is unique in its target", name, target, strings.Join(on, " and "))
	}
	if len(on) == 0 {
		return "", nil
	}
	return on[0], nil
}

func (f *runFolder) load() error {
	raw, err := os.ReadFile(filepath.Join(f.dir, settingsFile))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &f.settings); err != nil || f.Line == "" || f.Seed == "" {
		return fmt.Errorf("could not read %s: it should say the run's line and seed", filepath.Join(f.dir, settingsFile))
	}
	return nil
}

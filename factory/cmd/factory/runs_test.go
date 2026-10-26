package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// targetWith makes a target with the lines, each an empty folder, and the
// seed beside it.
func targetWith(t *testing.T, lines ...string) (target, seed string) {
	t.Helper()
	dir := t.TempDir()
	target, seed = filepath.Join(dir, "target"), filepath.Join(dir, "seed.md")
	for _, line := range lines {
		if err := os.MkdirAll(filepath.Join(target, linesFolder, line), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(seed, []byte("Build a game.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return target, seed
}

// A run is found by its target and its name, whichever line it is on, and
// remembers its line and its seed.
func TestARunIsFoundOnWhicheverLineItIsOn(t *testing.T) {
	target, seed := targetWith(t, "careful", "quick")
	first, err := invocation{run: "snake", target: target, line: "quick", seed: seed}.openRun()
	if err != nil {
		t.Fatal(err)
	}
	if first.started {
		t.Error("a run nobody has started is said to have started")
	}
	if err := first.save(); err != nil {
		t.Fatal(err)
	}

	again, err := invocation{run: "snake", target: target}.openRun()
	if err != nil {
		t.Fatal(err)
	}
	if !again.started || again.Line != "quick" || again.seedPath() != seed {
		t.Errorf("the run was found on line %q with seed %q, started %v; want quick and %s", again.Line, again.seedPath(), again.started, seed)
	}
	if want := filepath.Join(target, linesFolder, "quick", "runs", "snake"); again.dir != want {
		t.Errorf("the run's folder is %s, not %s", again.dir, want)
	}
}

// The record of a run is the same wherever the target is cloned: the seed is
// kept as a path from the target.
func TestARunKeepsItsSeedAsAPathFromTheTarget(t *testing.T) {
	target, seed := targetWith(t, "careful")
	f, err := invocation{run: "r", target: target, line: "careful", seed: seed}.openRun()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("..", "seed.md"); f.Seed != want {
		t.Errorf("the seed is kept as %q, not %q", f.Seed, want)
	}
}

func TestARunsNameIsUniqueInItsTarget(t *testing.T) {
	target, seed := targetWith(t, "careful", "quick")
	for _, line := range []string{"careful", "quick"} {
		f := runFolder{target: target, name: "r", settings: settings{line, seed}, dir: filepath.Join(target, linesFolder, line, "runs", "r")}
		if err := f.save(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := (invocation{run: "r", target: target}).openRun(); err == nil || !strings.Contains(err.Error(), "unique") {
		t.Errorf("a name on two lines was accepted (%v)", err)
	}
}

func TestARunNeedsATargetAndAName(t *testing.T) {
	target, seed := targetWith(t, "careful")
	for name, in := range map[string]invocation{
		"no target":               {run: "r", line: "careful", seed: seed},
		"no name":                 {target: target, line: "careful", seed: seed},
		"a name with a path":      {run: "../r", target: target, line: "careful", seed: seed},
		"no line":                 {run: "r", target: target, seed: seed},
		"no seed":                 {run: "r", target: target, line: "careful"},
		"a line the target lacks": {run: "r", target: target, line: "quick", seed: seed},
	} {
		if _, err := in.openRun(); err == nil {
			t.Errorf("%s: the run was opened", name)
		}
	}
}

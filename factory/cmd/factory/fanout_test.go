package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A fan out is read from the line: a machine with several unlabelled edges
// out, whose branches each have one unlabelled edge to the same machine.
func TestAFanOutIsReadFromTheLine(t *testing.T) {
	line, err := parseAssemblyLine([]byte(`digraph {
		start -> planner
		planner -> doer [label="not complete"]
		planner -> finish [label=complete]
		doer -> a
		doer -> b
		a -> meet
		b -> meet
		meet -> planner
	}`))
	if err != nil {
		t.Fatal(err)
	}
	branches := line.branches("doer")
	if !slices.Equal(branches, []string{"a", "b"}) {
		t.Errorf("doer fans out to %q, not a and b", branches)
	}
	if meet, ok := line.meeting(branches); !ok || meet != "meet" {
		t.Errorf("the branches meet at %q (%v), not at meet", meet, ok)
	}
	for _, node := range []string{"start", "planner", "a", "meet"} {
		if got := line.branches(node); got != nil {
			t.Errorf("%s fans out to %q", node, got)
		}
	}
}

// Harnesses that stand in for machines in a fan out. Each notes its process,
// and that of a child it starts, in $FANOUT_STATE/pids. A slow one keeps
// working, child and all, until it is stopped; a failing one waits until
// both slow ones have begun, so that it fails while they are at work.
const (
	slowHarness = `#!/bin/sh
echo $$ >> "$FANOUT_STATE/pids"
sleep 300 &
echo $! >> "$FANOUT_STATE/pids"
echo begun >> "$FANOUT_STATE/begun"
wait
`
	waitForTheSlowOnes = `#!/bin/sh
while [ "$(cat "$FANOUT_STATE/begun" 2>/dev/null | wc -l)" -lt 2 ]; do sleep 0.01; done
`
	proseHarness = waitForTheSlowOnes + `echo "I looked at the work and it seems fine."
`
	leavesAChildHarness = `#!/bin/sh
sleep 300 > /dev/null 2>&1 &
echo $! >> "$FANOUT_STATE/pids"
echo '{"satisfied": true}'
`
)

func TestABranchThatFailsStopsTheOthers(t *testing.T) {
	for name, failing := range map[string]string{
		"answers in prose": proseHarness,
		"cannot be run":    "",
	} {
		t.Run(name, func(t *testing.T) {
			state := t.TempDir()
			t.Setenv("FANOUT_STATE", state)
			// With no script, b's harness is a path where no program
			// exists, and the others may not have begun when it fails.
			b := machine{name: "b", dir: t.TempDir(), harness: filepath.Join(state, "no-such-harness"), model: "p/m"}
			if failing != "" {
				b = harnessed(t, "b", failing)
			}
			line, err := parseAssemblyLine([]byte(`digraph { doer -> a; doer -> b; doer -> c; a -> meet; b -> meet; c -> meet }`))
			if err != nil {
				t.Fatal(err)
			}
			r := &run{line: line, machines: map[string]machine{
				"a": harnessed(t, "a", slowHarness),
				"b": b,
				"c": harnessed(t, "c", slowHarness),
			}, folder: runFolder{target: t.TempDir()}}
			goroutines := runtime.NumGoroutine()

			var answers []answer
			within(t, 10*time.Second, "the fan out to stop", func() {
				answers, err = r.fanOut(context.Background(), "doer", []string{"a", "b", "c"}, briefing{})
			})
			if err == nil {
				t.Fatalf("the fan out succeeded with %v", answers)
			}
			for _, want := range []string{"a branch from doer failed", "the b"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the error does not say %q: %v", want, err)
				}
			}
			noneRunning(t, state)
			noGoroutinesLeft(t, goroutines)
		})
	}
}

// Stopping the factory stops the machine it is running, and whatever the
// machine started.
func TestStoppingTheFactoryStopsItsMachine(t *testing.T) {
	state := t.TempDir()
	t.Setenv("FANOUT_STATE", state)
	m := harnessed(t, "slow", slowHarness)
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := m.run(ctx, t.TempDir(), "do it")
		done <- err
	}()
	waitForFile(t, filepath.Join(state, "begun"))
	stop()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "was stopped") {
			t.Errorf("a stopped machine returned %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the machine was still running 10s after the factory stopped it")
	}
	noneRunning(t, state)
}

// A machine that answers leaves nothing running, and what it left running
// does not keep the factory waiting.
func TestAMachineLeavesNothingRunning(t *testing.T) {
	state := t.TempDir()
	t.Setenv("FANOUT_STATE", state)
	m := harnessed(t, "quick", leavesAChildHarness)
	var answer string
	var err error
	within(t, 5*time.Second, "the machine's call to return", func() {
		answer, err = m.run(context.Background(), t.TempDir(), "do it")
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(answer, `"satisfied"`) {
		t.Errorf("the machine answered %q", answer)
	}
	noneRunning(t, state)
}

// harnessed is a machine whose harness is the script, in a folder of its own.
func harnessed(t *testing.T, name, script string) machine {
	t.Helper()
	dir := t.TempDir()
	harness := filepath.Join(dir, "harness")
	if err := os.WriteFile(harness, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return machine{name: name, dir: dir, harness: harness, model: "p/m"}
}

// within fails the test if f takes longer than the limit.
func within(t *testing.T, limit time.Duration, what string, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(limit):
		t.Fatalf("waited %v for %s", limit, what)
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(path); err == nil {
			return
		}
	}
	t.Fatalf("%s never appeared", path)
}

// noneRunning checks every process noted in the state's pids has gone.
func noneRunning(t *testing.T, state string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(state, "pids"))
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range strings.Fields(string(raw)) {
		pid, err := strconv.Atoi(field)
		if err != nil {
			t.Fatal(err)
		}
		// A killed process another parent has yet to reap is gone too.
		gone := func() bool { return syscall.Kill(pid, 0) != nil || zombie(pid) }
		for deadline := time.Now().Add(2 * time.Second); !gone() && time.Now().Before(deadline); {
			time.Sleep(10 * time.Millisecond)
		}
		if !gone() {
			syscall.Kill(pid, syscall.SIGKILL)
			t.Errorf("process %d was still running", pid)
		}
	}
}

// zombie says whether the process has exited and not been reaped, where
// the system says (Linux's /proc).
func zombie(pid int) bool {
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return false
	}
	_, after, _ := strings.Cut(string(stat), ") ")
	return strings.HasPrefix(after, "Z")
}

func noGoroutinesLeft(t *testing.T, before int) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); runtime.NumGoroutine() > before && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
	}
	if now := runtime.NumGoroutine(); now > before {
		t.Errorf("%d goroutines were left running after the fan out (%d before)", now-before, before)
	}
}

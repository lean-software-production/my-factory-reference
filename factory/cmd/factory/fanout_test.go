package main

import (
	"context"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lean-software-production/my-factory-reference/factory/internal/acptest"
	"github.com/lean-software-production/my-factory-reference/factory/internal/machine"
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

func TestABranchThatFailsStopsTheOthers(t *testing.T) {
	acptest.NeedSetsid(t)
	for name, failing := range map[string]string{
		"answers in prose": acptest.Script(acptest.WaitForTheSlowOnes, "I looked at the work and it seems fine."),
		"cannot be run":    "",
	} {
		t.Run(name, func(t *testing.T) {
			state := t.TempDir()
			t.Setenv("FANOUT_STATE", state)
			// With no script, b's harness is a path where no program
			// exists, and the others may not have begun when it fails.
			b := machine.Machine{Name: "b", Dir: t.TempDir(), Harness: filepath.Join(state, "no-such-harness"), Model: "p/m"}
			if failing != "" {
				b = harnessed(t, "b", failing)
			}
			line, err := parseAssemblyLine([]byte(`digraph { doer -> a; doer -> b; doer -> c; a -> meet; b -> meet; c -> meet }`))
			if err != nil {
				t.Fatal(err)
			}
			// c works the way pi does: what it runs is beyond its process
			// group, and it cleans up when it is asked to stop.
			r := &run{line: line, machines: map[string]machine.Machine{
				"a": harnessed(t, "a", acptest.Script(acptest.Slow)),
				"b": b,
				"c": harnessed(t, "c", acptest.Script(acptest.PiLike)),
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
			acptest.NoneRunning(t, state)
			noGoroutinesLeft(t, goroutines)
		})
	}
}

// harnessed is a machine whose harness is the script, in a folder of its own.
func harnessed(t *testing.T, name, script string) machine.Machine {
	t.Helper()
	return machine.Machine{Name: name, Dir: t.TempDir(), Harness: acptest.Harness(t, "harness", script), Model: "p/m"}
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

func noGoroutinesLeft(t *testing.T, before int) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); runtime.NumGoroutine() > before && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
	}
	if now := runtime.NumGoroutine(); now > before {
		t.Errorf("%d goroutines were left running after the fan out (%d before)", now-before, before)
	}
}

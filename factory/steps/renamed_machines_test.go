package steps

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// What a machine is for is in the line and its AGENTS.md, not in its name.
// These runs rename the three machines (lead, builder, checker) and check
// the line still commits each task's work, sends the checker's findings back
// to the builder, shows the checker the work, and limits attempts.

func renamedWorld(t *testing.T) *world {
	t.Helper()
	w := newWorld()
	t.Cleanup(w.tearDown)
	steps := []func() error{
		w.setUp,
		func() error { w.useTarget("games/new-game"); return nil },
		func() error { return w.writeSeed(tetrisSeed) },
		func() error { return w.writePlan(0) },
		func() error {
			for from, to := range map[string]string{"planner": "lead", "doer": "builder", "validator": "checker"} {
				if err := os.Rename(w.machineDir(from), w.machineDir(to)); err != nil {
					return err
				}
			}
			return w.editLine(func(line string) string {
				for from, to := range map[string]string{"planner": "lead", "doer": "builder", "validator": "checker"} {
					line = strings.ReplaceAll(line, from, to)
				}
				return line
			})
		},
	}
	for _, step := range steps {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	return w
}

func TestRenamedMachinesFinishEveryTaskAndGetTheirFindings(t *testing.T) {
	w := renamedWorld(t)
	w.does.Rejects, w.does.Finding = 1, finding
	if err := w.runToSuccess(); err != nil {
		t.Fatal(err)
	}

	for name, check := range map[string]func() error{
		"every task done":          func() error { return w.planShows(0, -1, true) },
		"three work commits":       func() error { return w.workCommits(3) },
		"one task in each commit":  w.eachWorkCommitHoldsOneTask,
		"findings back to builder": func() error { return w.someCallWasGiven("doer", finding) },
		"checker shown the work":   func() error { return w.someCallWasGiven("validator", "one.txt") },
		"builder called 4 times": func() error {
			if calls, _ := w.callsTo("doer"); len(calls) != 4 {
				return fmt.Errorf("the builder was called %d times", len(calls))
			}
			return nil
		},
	} {
		if err := check(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	// A commit is named for the task its builder did, not for the default.
	log, err := git(w.repo, "log", "--format=%s", w.headBefore+"..HEAD")
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range threeTasks {
		if !strings.Contains(log, task) {
			t.Errorf("no commit is named for %q:\n%s", task, log)
		}
	}
}

func TestRenamedMachinesStillLimitAttempts(t *testing.T) {
	w := renamedWorld(t)
	w.attempts = 3
	w.does.Rejects, w.does.Finding = -1, finding
	if err := w.run(); err != nil {
		t.Fatal(err)
	}
	if err := w.reports("a task hit its limit"); err != nil {
		t.Error(err)
	}
	if calls, _ := w.callsTo("doer"); len(calls) != 3 {
		t.Errorf("the builder was called %d times, not 3", len(calls))
	}
	head, err := git(w.repo, "rev-parse", "HEAD")
	if err != nil || head != w.headBefore {
		t.Errorf("a task that gave up was committed (%v)", err)
	}
}

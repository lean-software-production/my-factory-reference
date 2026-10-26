package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// From homework 7 the factory is a daemon: starting a run returns while it
// runs, and runs, and the factory, are stopped by commands of their own.
// A machine an example sees part-way through its job is held there by the
// double until a step lets it go, or the run is stopped or run to the end.

// waitLimit is how long a step waits for something to happen.
const waitLimit = 10 * time.Second

func (w *world) registerDaemonSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I start the "([^"]*)" run$`, func(name string) error {
		// The doer does not finish its first attempt until a step says so.
		if err := w.hold("doer"); err != nil {
			return err
		}
		return w.startNamed(name)
	})
	sc.Step(`^the "([^"]*)" run is running, with the doer part-way through an attempt(?:, having said "([^"]*)")?$`, w.runningWithTheDoerHeld)
	sc.Step(`^the "([^"]*)" run is running, with every reviewer part-way through$`, w.runningWithTheReviewersHeld)
	sc.Step(`^I (?:stop|have stopped) the "([^"]*)" run$`, w.stopRun)
	sc.Step(`^I stop the factory$`, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), doubleRunLimit)
		defer cancel()
		if out, err := w.factory(ctx, "stop"); err != nil {
			return fmt.Errorf("stopping the factory: %v\n%s", err, out)
		}
		return nil
	})
	sc.Step(`^the "([^"]*)" run has run to the end$`, w.runToTheEnd)

	sc.Step(`^the command has returned while the "([^"]*)" run is running$`, func(name string) error {
		if w.timedOut || w.exitErr != nil {
			return fmt.Errorf("starting the %q run did not return (%v):\n%s", name, w.exitErr, w.output)
		}
		if !strings.Contains(w.output, "started run "+name) {
			return fmt.Errorf("starting the run said:\n%s", w.output)
		}
		return w.runIsRunning(name, true)
	})
	sc.Step(`^the factory is (running|not running)$`, func(state string) error {
		status, err := w.status()
		if err != nil {
			return err
		}
		if running := factoryRunning.MatchString(status); running != (state == "running") {
			return fmt.Errorf("the factory is not %s; it says:\n%s", state, status)
		}
		return nil
	})
	sc.Step(`^the "([^"]*)" run is running$`, func(name string) error { return w.runIsRunning(name, true) })
	sc.Step(`^the "([^"]*)" run is not running$`, func(name string) error {
		if err := w.runIsRunning(name, false); err != nil {
			return err
		}
		// Nor is any machine it ran.
		return w.noMachineRunning()
	})
}

var (
	factoryRunning = regexp.MustCompile(`(?m)^the factory is running \(process \d+\)$`)
	runRunning     = regexp.MustCompile(`(?m)^run (\S+) is running, in (.+)$`)
)

// status is what the factory says of itself.
func (w *world) status() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), waitLimit)
	defer cancel()
	out, err := w.factory(ctx, "status")
	if err != nil {
		return "", fmt.Errorf("asking the factory how it is: %v\n%s", err, out)
	}
	return out, nil
}

// runIsRunning checks whether the factory says the run is running, in its
// target.
func (w *world) runIsRunning(name string, want bool) error {
	r, err := w.runNamed(name)
	if err != nil {
		return err
	}
	status, err := w.status()
	if err != nil {
		return err
	}
	running := false
	if m := runRunning.FindStringSubmatch(status); m != nil && m[1] == name {
		running = samePath(m[2], w.fromRepo(r.target))
	}
	if running != want {
		return fmt.Errorf("expected the %q run running: %v, but the factory says:\n%s", name, want, status)
	}
	return nil
}

func samePath(a, b string) bool {
	if real, err := filepath.EvalSymlinks(a); err == nil {
		a = real
	}
	if real, err := filepath.EvalSymlinks(b); err == nil {
		b = real
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// startNamed has the factory start the run, and fails unless it started.
func (w *world) startNamed(name string) error {
	r, err := w.runNamed(name)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), doubleRunLimit)
	defer cancel()
	return w.start(ctx, r, invocation{})
}

func (w *world) startedOrFail(name string) error {
	if err := w.startNamed(name); err != nil {
		return err
	}
	if w.exitErr != nil || w.timedOut {
		return fmt.Errorf("the %q run did not start (%v):\n%s", name, w.exitErr, w.output)
	}
	return nil
}

// runningWithTheDoerHeld starts the run with the doer held part-way through
// its first attempt, having said the words, if any, and waits until it is.
func (w *world) runningWithTheDoerHeld(name, said string) error {
	if said != "" && !slices.Contains(w.does.DoerSays, said) {
		w.does.DoerSays = append(w.does.DoerSays, said)
	}
	if err := w.hold("doer"); err != nil {
		return err
	}
	if err := w.startedOrFail(name); err != nil {
		return err
	}
	return w.waitUntilHeld("doer")
}

// runningWithTheReviewersHeld starts the run with every reviewer held
// part-way through, and waits until all three are.
func (w *world) runningWithTheReviewersHeld(name string) error {
	if err := w.hold("reviewer"); err != nil {
		return err
	}
	if err := w.startedOrFail(name); err != nil {
		return err
	}
	return w.waitUntilHeld(reviewers...)
}

// hold has the double hold the machines in that role part-way through
// their job, until the hold is let go.
func (w *world) hold(role string) error {
	return os.WriteFile(filepath.Join(w.doubleDir, "hold-"+role), nil, 0o644)
}

// releaseHolds lets every held machine go on.
func (w *world) releaseHolds() error {
	for _, role := range []string{"doer", "reviewer"} {
		if err := os.Remove(filepath.Join(w.doubleDir, "hold-"+role)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// waitUntilHeld waits until each of the machines has been held.
func (w *world) waitUntilHeld(machines ...string) error {
	return waitFor(waitLimit, func() error {
		raw, _ := os.ReadFile(filepath.Join(w.doubleDir, "holds.log"))
		var held []string
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
			if fields := strings.Fields(line); len(fields) == 2 {
				held = append(held, fields[1])
			}
		}
		for _, machine := range machines {
			if !slices.Contains(held, machine) {
				return fmt.Errorf("%s is not part-way through its job (held: %v)\n%s", machine, held, w.output)
			}
		}
		return nil
	})
}

// stopRun has the factory stop the run, which returns once it has.
func (w *world) stopRun(name string) error {
	r, err := w.runNamed(name)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), doubleRunLimit)
	defer cancel()
	if out, err := w.factory(ctx, "stop", name, "--target", r.target); err != nil {
		return fmt.Errorf("stopping the %q run: %v\n%s", name, err, out)
	}
	return nil
}

// runToTheEnd lets the run go on to its end, and waits until it has: what
// is being watched is watched to the end.
func (w *world) runToTheEnd(name string) error {
	r, err := w.runNamed(name)
	if err != nil {
		return err
	}
	if err := w.releaseHolds(); err != nil {
		return err
	}
	if w.watching != nil {
		if err := w.watching.wait(doubleRunLimit); err != nil {
			return fmt.Errorf("the %q run did not finish: %v\n%s", name, err, w.watching.output())
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), doubleRunLimit)
	defer cancel()
	if out, err := w.factory(ctx, "watch", name, "--target", r.target); err != nil {
		return fmt.Errorf("the %q run did not finish: %v\n%s", name, err, out)
	}
	return nil
}

// waitFor waits until check passes, or the limit, when it fails with the
// check's latest reason.
func waitFor(limit time.Duration, check func() error) error {
	deadline := time.Now().Add(limit)
	for {
		err := check()
		if err == nil || time.Now().After(deadline) {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
}

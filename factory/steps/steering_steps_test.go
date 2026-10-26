package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

// From homework 8 a machine can be told something while it works: the
// factory passes the message on to its running turn. A double that is
// told something says "heard: <message>" at the point it heard it, so a
// step can tell what it heard before and after what it said.

// heardPrefix begins the line a double says when it hears a message.
const heardPrefix = "heard: "

func (w *world) registerSteeringSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I say "([^"]*)" to (the doer|the validator|reviewer_\d)$`, w.say)
	sc.Step(`^the doer finishes its attempt$`, func() error { return w.letGo("doer") })
	sc.Step(`^the reviewers finish$`, func() error { return w.letGo("reviewer") })
	sc.Step(`^the factory has run the "([^"]*)" run$`, func(name string) error {
		r, err := w.runNamed(name)
		if err != nil {
			return err
		}
		return w.runRunToSuccess(r, invocation{})
	})
	sc.Step(`^it says that the doer is what is running$`, func() error {
		return w.reportsMatching(`(^|\W)doer is what is running`)
	})
	sc.Step(`^it says that nothing is running$`, func() error { return w.reports("nothing is running") })

	sc.Step(`^the doer heard "([^"]*)" before it said "([^"]*)"$`, func(message, words string) error {
		calls, err := w.saidInEachCall("doer")
		if err != nil {
			return err
		}
		for _, said := range calls {
			heard := slices.Index(said, heardPrefix+message)
			if heard >= 0 && slices.Contains(said[heard:], words) {
				return nil
			}
		}
		return fmt.Errorf("no call to the doer heard %q before it said %q: %q", message, words, calls)
	})
	sc.Step(`^(reviewer_\d) heard "([^"]*)"$`, func(machine, message string) error {
		return w.heard(machine, message, 1)
	})
	sc.Step(`^the doer heard "([^"]*)" once$`, func(message string) error { return w.heard("doer", message, 1) })
	sc.Step(`^the validator did not hear "([^"]*)"$`, func(message string) error {
		return w.heard("validator", message, 0)
	})
	sc.Step(`^it shows that the doer was told "([^"]*)"$`, func(message string) error {
		for _, ev := range w.record {
			if ev.Kind == "told" && ev.Machine == "doer" && ev.Text == message {
				return nil
			}
		}
		return fmt.Errorf("the record does not show the doer told %q: %+v", message, w.record)
	})
	sc.Step(`^neither (reviewer_\d) nor (reviewer_\d) heard it$`, func(one, other string) error {
		for _, machine := range []string{one, other} {
			if err := w.heard(machine, w.told, 0); err != nil {
				return err
			}
		}
		return nil
	})
}

// heard checks how many of the calls to the machine in that folder heard
// the message: were told it as they worked, or were given it in their
// prompt.
func (w *world) heard(machine, message string, want int) error {
	calls, err := w.callsToMachine(machine)
	if err != nil {
		return err
	}
	said, err := w.saidInEachCall(machine)
	if err != nil {
		return err
	}
	if len(calls) == 0 {
		return fmt.Errorf("%s was never called", machine)
	}
	n := 0
	for i, c := range calls {
		if slices.Contains(said[i], heardPrefix+message) || strings.Contains(c.prompt, message) {
			n++
		}
	}
	if n != want {
		return fmt.Errorf("%d of the %d calls to %s heard %q, not %d: %q\n%s", n, len(calls), machine, message, want, said, w.output)
	}
	return nil
}

// say has the factory pass the words to the machine, in the current run,
// through bin/factory. Whether it delivered them or refused is for later
// steps to check.
func (w *world) say(words, to string) error {
	r := w.current()
	machine := strings.TrimPrefix(to, "the ")
	ctx, cancel := context.WithTimeout(context.Background(), waitLimit)
	defer cancel()
	w.told = words
	w.output, w.exitErr = w.factory(ctx, "say", r.name, "--target", r.target, machine, words)
	w.timedOut = ctx.Err() != nil
	return nil
}

// letGo lets the machines in that role, held part-way through their job,
// go on.
func (w *world) letGo(role string) error {
	if err := os.Remove(filepath.Join(w.doubleDir, "hold-"+role)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// saidInEachCall is what each call to the machine in that folder said, a
// list of lines for each call, in order.
func (w *world) saidInEachCall(machine string) ([][]string, error) {
	calls, err := w.calls()
	if err != nil {
		return nil, err
	}
	var said [][]string
	for n, c := range calls {
		if c.machine != machine {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(w.doubleDir, "said", fmt.Sprintf("%d.txt", n+1)))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		said = append(said, strings.Split(strings.TrimSpace(string(raw)), "\n"))
	}
	return said, nil
}

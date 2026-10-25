package steps

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

func (w *world) registerRunSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the factory runs$`, w.run)
	sc.Step(`^the factory builds the target "([^"]*)" to completion$`, func(target string) error {
		w.useTarget(target)
		return w.runToSuccess()
	})
	sc.Step(`^the factory builds the same target using its absolute path$`, func() error {
		w.useTarget(w.targetDir())
		return w.runToSuccess()
	})

	sc.Step(`^the factory has stopped$`, func() error {
		if w.timedOut {
			return fmt.Errorf("the factory was still running when it was stopped:\n%s", w.output)
		}
		return nil
	})

	sc.Step(`^it reports that it could not run the doer$`, func() error {
		return w.reports("could not run the doer")
	})
	sc.Step(`^it reports that it could not read the validator's result$`, func() error {
		return w.reports("could not read the validator's result")
	})
	sc.Step(`^it reports that a task hit its limit$`, func() error { return w.reports("a task hit its limit") })
	sc.Step(`^it reports that the result of validator has no field "([^"]*)"$`, func(field string) error {
		return w.reports(fmt.Sprintf("the result of validator has no field %q", field))
	})
	sc.Step(`^it reports that there is no seed$`, func() error { return w.reports("there is no seed") })
	sc.Step(`^it reports that a target is required$`, func() error { return w.reports("a target is required") })

	sc.Step(`^pi has been called$`, func() error { return w.calledBy("pi", true) })
	sc.Step(`^pi has not been called$`, func() error { return w.calledBy("pi", false) })
	sc.Step(`^the doer's chosen harness has been called$`, func() error {
		return w.calledBy(filepath.Base(agentDouble), true)
	})
	sc.Step(`^no agent has been called$`, func() error { return w.calledTimes(0) })
	sc.Step(`^the validator has not been called$`, func() error {
		calls, err := w.callsTo("validator")
		if err != nil {
			return err
		}
		if len(calls) != 0 {
			return fmt.Errorf("expected the validator not to be called, but it was called %d times\n%s", len(calls), w.output)
		}
		return nil
	})
	sc.Step(`^the planner was called before the doer$`, func() error {
		calls, err := w.calls()
		if err != nil {
			return err
		}
		for _, c := range calls {
			if c.role == "doer" {
				return fmt.Errorf("the doer was called before the planner\n%s", w.output)
			}
			if c.role == "planner" {
				return nil
			}
		}
		return fmt.Errorf("the planner was never called\n%s", w.output)
	})
	sc.Step(`^the doer has (not been called|been called (?:once|twice|three times|four times))$`, func(times string) error {
		calls, err := w.callsTo("doer")
		if err != nil {
			return err
		}
		want := map[string]int{"not been called": 0, "been called once": 1, "been called twice": 2, "been called three times": 3, "been called four times": 4}[times]
		if len(calls) != want {
			return fmt.Errorf("expected the doer to have %s, but it was called %d times\n%s", times, len(calls), w.output)
		}
		return nil
	})

	sc.Step(`^the doer was pointed at the plan and at the seed$`, func() error {
		prompt, err := w.lastPrompt("doer")
		if err != nil {
			return err
		}
		for _, path := range []string{w.planPath(), w.seedPath()} {
			if !strings.Contains(prompt, path) {
				return fmt.Errorf("the doer was not given %s; it was given:\n%s", path, prompt)
			}
		}
		return nil
	})
	sc.Step(`^the validator was given the work for the second task$`, func() error {
		return w.someCallWasGiven("validator", w.planTasks[1]+".txt")
	})
	sc.Step(`^the doer was given the validator's findings$`, func() error {
		return w.someCallWasGiven("doer", finding)
	})
	sc.Step(`^the (doer|validator) was given "([^"]*)"$`, func(role, words string) error {
		w.subject = role
		return w.wasGiven(role, words)
	})
	sc.Step(`^it was not given "([^"]*)"$`, func(words string) error {
		if w.subject == "" {
			return fmt.Errorf("no machine has been said to be given anything")
		}
		prompt, err := w.lastPrompt(w.subject)
		if err != nil {
			return err
		}
		if strings.Contains(prompt, words) {
			return fmt.Errorf("the %s was given %q:\n%s", w.subject, words, prompt)
		}
		return nil
	})
	sc.Step(`^it was not given the work for the first task$`, func() error {
		calls, err := w.callsTo("validator")
		if err != nil {
			return err
		}
		file := w.planTasks[0] + ".txt"
		for _, c := range calls {
			if strings.Contains(c.prompt, file) {
				return fmt.Errorf("the validator was given %s; it was given:\n%s", file, c.prompt)
			}
		}
		return nil
	})
	sc.Step(`^the (planner|validator) was asked for a result with the fields? "([^"]*)"(?: and "([^"]*)")?$`,
		func(role, field, another string) error {
			prompt, err := w.lastPrompt(role)
			if err != nil {
				return err
			}
			for _, field := range []string{field, another} {
				if field != "" && !strings.Contains(prompt, `"`+field+`"`) {
					return fmt.Errorf("the %s was not asked for the field %q; it was given:\n%s", role, field, prompt)
				}
			}
			return nil
		})
}

// someCallWasGiven checks that the machine, in one of its calls, was given
// the words.
func (w *world) someCallWasGiven(role, words string) error {
	calls, err := w.callsTo(role)
	if err != nil {
		return err
	}
	for _, c := range calls {
		if strings.Contains(c.prompt, words) {
			return nil
		}
	}
	return fmt.Errorf("the %s was not given %q in any of its %d calls\n%s", role, words, len(calls), w.output)
}

// wasGiven checks the machine's instructions and prompt, the latest time it
// was called, hold the words.
func (w *world) wasGiven(role, words string) error {
	prompt, err := w.lastPrompt(role)
	if err != nil {
		return err
	}
	if !strings.Contains(prompt, words) {
		return fmt.Errorf("the %s was not given %q; it was given:\n%s", role, words, prompt)
	}
	return nil
}

func (w *world) calledBy(name string, want bool) error {
	calls, err := w.calls()
	if err != nil {
		return err
	}
	var names []string
	for _, c := range calls {
		names = append(names, c.name)
	}
	if got := slices.Contains(names, name); got != want {
		return fmt.Errorf("expected %s called: %v, but the calls were %v\n%s", name, want, names, w.output)
	}
	return nil
}

func (w *world) calledTimes(n int) error {
	calls, err := w.calls()
	if err != nil {
		return err
	}
	if len(calls) != n {
		return fmt.Errorf("expected %d agent calls, got %v\n%s", n, calls, w.output)
	}
	return nil
}

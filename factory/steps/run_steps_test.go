package steps

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

func (w *world) registerRunSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the factory runs one pass$`, func() error { return w.run(false) })
	sc.Step(`^the factory runs to completion$`, func() error { return w.run(true) })
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
	sc.Step(`^it reports that the pass hit its limit$`, func() error { return w.reports("the pass hit its limit") })
	sc.Step(`^it reports that there is no seed$`, func() error { return w.reports("there is no seed") })
	sc.Step(`^it reports that a target is required$`, func() error { return w.reports("a target is required") })

	sc.Step(`^pi has been called$`, func() error { return w.calledBy("pi", true) })
	sc.Step(`^pi has not been called$`, func() error { return w.calledBy("pi", false) })
	sc.Step(`^the doer's chosen harness has been called$`, func() error {
		return w.calledBy(filepath.Base(agentDouble), true)
	})
	sc.Step(`^no agent has been called$`, func() error { return w.calledTimes(0) })
	sc.Step(`^the doer has (not been called|been called (?:once|twice|three times))$`, func(times string) error {
		calls, err := w.callsTo("doer")
		if err != nil {
			return err
		}
		want := map[string]int{"not been called": 0, "been called once": 1, "been called twice": 2, "been called three times": 3}[times]
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
		return w.validatorWasGiven(w.planTasks[1] + ".txt")
	})
	sc.Step(`^the doer was given the validator's findings$`, func() error {
		prompt, err := w.lastPrompt("doer")
		if err != nil {
			return err
		}
		if !strings.Contains(prompt, finding) {
			return fmt.Errorf("the doer was not given the finding %q; it was given:\n%s", finding, prompt)
		}
		return nil
	})
	sc.Step(`^the validator was given "([^"]*)"$`, w.validatorWasGiven)
	sc.Step(`^it was not given the work for the first task$`, func() error {
		prompt, err := w.lastPrompt("validator")
		if err != nil {
			return err
		}
		if file := w.planTasks[0] + ".txt"; strings.Contains(prompt, file) {
			return fmt.Errorf("the validator was given %s; it was given:\n%s", file, prompt)
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

func (w *world) validatorWasGiven(words string) error {
	prompt, err := w.lastPrompt("validator")
	if err != nil {
		return err
	}
	if !strings.Contains(prompt, words) {
		return fmt.Errorf("the validator was not given %q; it was given:\n%s", words, prompt)
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

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

	sc.Step(`^it reports that it could not run the agent$`, func() error {
		return w.reports("could not run the agent")
	})
	sc.Step(`^it reports that it could not read the agent's result$`, func() error {
		return w.reports("could not read the agent's result")
	})
	sc.Step(`^it reports that there is no seed$`, func() error { return w.reports("there is no seed") })
	sc.Step(`^it reports that a target is required$`, func() error { return w.reports("a target is required") })

	sc.Step(`^pi has been called$`, func() error { return w.calledBy("pi", true) })
	sc.Step(`^pi has not been called$`, func() error { return w.calledBy("pi", false) })
	sc.Step(`^the chosen agent has been called$`, func() error {
		return w.calledBy(filepath.Base(agentDouble), true)
	})
	sc.Step(`^no agent has been called$`, func() error { return w.calledTimes(0) })
	sc.Step(`^the agent has been called once$`, func() error { return w.calledTimes(1) })

	sc.Step(`^the agent was pointed at the plan and at the seed$`, func() error {
		prompt, err := w.lastPrompt()
		if err != nil {
			return err
		}
		for _, path := range []string{w.planPath(), w.seedPath()} {
			if !strings.Contains(prompt, path) {
				return fmt.Errorf("the agent was not given %s; it was given:\n%s", path, prompt)
			}
		}
		return nil
	})
	sc.Step(`^the agent was asked for a result with the field "([^"]*)"$`, func(field string) error {
		prompt, err := w.lastPrompt()
		if err != nil {
			return err
		}
		if !strings.Contains(prompt, `"`+field+`"`) {
			return fmt.Errorf("the agent was not asked for the field %q; it was given:\n%s", field, prompt)
		}
		return nil
	})
}

func (w *world) calledBy(name string, want bool) error {
	calls, err := w.calls()
	if err != nil {
		return err
	}
	if got := slices.Contains(calls, name); got != want {
		return fmt.Errorf("expected %s called: %v, but the calls were %v\n%s", name, want, calls, w.output)
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

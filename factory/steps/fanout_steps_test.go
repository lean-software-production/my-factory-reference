package steps

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
)

// The three big brains: three reviewers, reviewer_1 to reviewer_3, and the
// synthesiser where their branches meet. They are the student's, on the
// three-big-brains line of their target (see sourceLines); a line in an
// example gets copies of them.
var (
	reviewers  = []string{"reviewer_1", "reviewer_2", "reviewer_3"}
	bigBrains  = append(reviewers[:len(reviewers):len(reviewers)], "synthesiser")
	brainsLine = "three-big-brains"
)

func (w *world) registerFanOutSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the line has the three big brains$`, func() error { return w.addBigBrains(w.currentLineDir()) })
	sc.Step(`^the three big brains has replaced the validator on the "([^"]*)" line$`, w.bigBrainsReplaceValidator)
	sc.Step(`^every reviewer is always satisfied$`, func() {})  // what the double does unless told otherwise
	sc.Step(`^the synthesiser is always satisfied$`, func() {}) // likewise
	sc.Step(`^no reviewer finishes until all three have begun$`, func() { w.does.ReviewersWait = true })
	sc.Step(`^a plan with one task, not done$`, func() error {
		w.planTasks = threeTasks[:1]
		w.current().tasks = w.planTasks
		return w.writePlanFor(w.current(), w.planTasks, 0)
	})

	sc.Step(`^every reviewer has been called once$`, func() error {
		for _, reviewer := range reviewers {
			if err := w.machineCalled(reviewer, 1); err != nil {
				return err
			}
		}
		return nil
	})
	sc.Step(`^the synthesiser has (not been called|been called once)$`, func(times string) error {
		return w.machineCalled("synthesiser", map[string]int{"not been called": 0, "been called once": 1}[times])
	})
	sc.Step(`^every reviewer was asked for a result$`, func() error {
		return w.everyReviewer(func(reviewer string, c call) error {
			// What the factory asked, after the machine's instructions.
			_, asked, _ := strings.Cut(c.prompt, "\n---\n")
			if !strings.Contains(asked, "your result") || !strings.Contains(asked, "JSON") {
				return fmt.Errorf("%s was not asked for a result; it was given:\n%s", reviewer, c.prompt)
			}
			return nil
		})
	})
	sc.Step(`^every reviewer is not satisfied, with a finding of its own: "report 1", "report 2" or "report 3"$`, func() {
		w.does.NumberedReports = true
	})
	sc.Step(`^the second reviewer is not satisfied, with the finding "([^"]*)"$`, func(finding string) {
		w.does.ReviewerFindings = map[string]string{"reviewer_2": finding}
	})
	sc.Step(`^the second reviewer answers in prose, with no result$`, func() { w.does.Prose = []string{"reviewer_2"} })
	sc.Step(`^the second reviewer cannot be run$`, func() error {
		return w.setHarness("reviewer_2", filepath.Join(w.ws, "no-such-harness"))
	})
	sc.Step(`^the synthesiser is never satisfied$`, func() { w.does.SynthesiserRejects, w.does.SynthesiserFinding = -1, finding })
	sc.Step(`^the synthesiser is not satisfied the first time$`, func() {
		w.does.SynthesiserRejects, w.does.SynthesiserFinding = 1, finding
	})
	sc.Step(`^the reviewers' lens is (\w+)$`, func(lens string) error {
		for _, reviewer := range reviewers {
			if err := w.setLensOf(reviewer, lens); err != nil {
				return err
			}
		}
		if w.realPi {
			return w.useFlawedPlan()
		}
		return nil
	})

	sc.Step(`^the synthesiser was given "([^"]*)"(?:, "([^"]*)" and "([^"]*)")?$`, func(a, b, c string) error {
		w.subject = "synthesiser"
		for _, words := range []string{a, b, c} {
			if words == "" {
				continue
			}
			if err := w.wasGiven("synthesiser", words); err != nil {
				return err
			}
		}
		return nil
	})
	sc.Step(`^the doer was given the synthesiser's findings$`, func() error {
		w.subject = "doer"
		return w.someCallWasGiven("doer", finding)
	})
	sc.Step(`^every reviewer was given the work for the second task$`, func() error {
		w.subject = "reviewer"
		work := w.planTasks[1] + ".txt"
		for _, reviewer := range reviewers {
			calls, err := w.callsToMachine(reviewer)
			if err != nil {
				return err
			}
			given := false
			for _, c := range calls {
				given = given || strings.Contains(c.prompt, work)
			}
			if !given {
				return fmt.Errorf("%s was not given %s in any of its %d calls\n%s", reviewer, work, len(calls), w.output)
			}
		}
		return nil
	})
	sc.Step(`^every reviewer was given "([^"]*)"$`, func(words string) error {
		w.subject = "reviewer"
		return w.everyReviewer(func(reviewer string, c call) error {
			if !strings.Contains(c.prompt, words) {
				return fmt.Errorf("%s was not given %q; it was given:\n%s", reviewer, words, c.prompt)
			}
			return nil
		})
	})
	sc.Step(`^every reviewer was given the same prompt$`, func() error {
		var first call
		return w.everyReviewer(func(reviewer string, c call) error {
			if reviewer == reviewers[0] {
				first = c
			} else if c.prompt != first.prompt {
				return fmt.Errorf("%s and %s were given different prompts:\n%s\n=====\n%s", reviewers[0], reviewer, first.prompt, c.prompt)
			}
			return nil
		})
	})
	sc.Step(`^the reviewers' findings are about (\w+)$`, func(lens string) error {
		found, err := w.findingsOf(reviewers...)
		if err != nil {
			return err
		}
		return w.findingsAbout(lens, found)
	})

	sc.Step(`^it reports that it could not read reviewer_2's result$`, func() error {
		return w.reportsMatching(`could not read (the )?reviewer_2's result`)
	})
	sc.Step(`^it reports that it could not run reviewer_2$`, func() error {
		return w.reportsMatching(`could not run (the )?reviewer_2\b`)
	})
	sc.Step(`^there is one new work commit$`, func() error { return w.workCommits(1) })

	sc.Step(`^it reports that the branches from (\w+) do not meet$`, func(from string) error {
		return w.reports(fmt.Sprintf("the branches from %s do not meet", from))
	})
}

// bigBrainsReplaceValidator puts the three big brains on the line in place
// of the validator: the doer's edge to the validator becomes an unlabelled
// edge to each reviewer, each reviewer has an unlabelled edge to the
// synthesiser, and the validator's edges become the synthesiser's.
func (w *world) bigBrainsReplaceValidator(line string) error {
	lineDir := w.lineDirIn(w.target, line)
	if err := w.addBigBrains(lineDir); err != nil {
		return err
	}
	if err := os.RemoveAll(w.machineDirOn(lineDir, "validator")); err != nil {
		return err
	}
	toValidator := regexp.MustCompile(`(?m)^(\s*)doer\s*->\s*validator\s*$`)
	fromValidator := regexp.MustCompile(`(?m)^(\s*)validator(\s*->)`)
	return w.editLine(line, func(graph string) string {
		var fan []string
		for _, reviewer := range reviewers {
			fan = append(fan, "${1}doer -> "+reviewer)
		}
		for _, reviewer := range reviewers {
			fan = append(fan, "${1}"+reviewer+" -> synthesiser")
		}
		graph = toValidator.ReplaceAllString(graph, strings.Join(fan, "\n"))
		return fromValidator.ReplaceAllString(graph, "${1}synthesiser${2}")
	})
}

// everyReviewer checks each reviewer's latest call.
func (w *world) everyReviewer(check func(reviewer string, latest call) error) error {
	for _, reviewer := range reviewers {
		calls, err := w.callsToMachine(reviewer)
		if err != nil {
			return err
		}
		if len(calls) == 0 {
			return fmt.Errorf("%s was never called\n%s", reviewer, w.output)
		}
		if err := check(reviewer, calls[len(calls)-1]); err != nil {
			return err
		}
	}
	return nil
}

// machineCalled checks the machine in that folder was called that many times.
func (w *world) machineCalled(machine string, want int) error {
	calls, err := w.callsToMachine(machine)
	if err != nil {
		return err
	}
	if len(calls) != want {
		return fmt.Errorf("expected %s to be called %d times, but it was called %d times\n%s", machine, want, len(calls), w.output)
	}
	return nil
}

// addBigBrains copies the three big brains' machines from the student's
// line into the line, with the double as their harness and a login each.
func (w *world) addBigBrains(lineDir string) error {
	from, err := filepath.Abs(filepath.Join(sourceLines, brainsLine))
	if err != nil {
		return err
	}
	files, err := lineFiles(from)
	if err != nil {
		return err
	}
	for _, machine := range bigBrains {
		copied := false
		for _, file := range files {
			if !strings.HasPrefix(file, "machines/"+machine+"/") {
				continue
			}
			to := filepath.Join(lineDir, file)
			if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
				return err
			}
			if err := copyFile(filepath.Join(from, file), to, 0o644); err != nil {
				return err
			}
			copied = true
		}
		if !copied {
			return fmt.Errorf("the %s line has no machine called %s", from, machine)
		}
		if err := w.setHarnessOn(lineDir, machine, agentDouble); err != nil {
			return err
		}
		if err := w.giveLoginOn(lineDir, machine); err != nil {
			return err
		}
	}
	return nil
}

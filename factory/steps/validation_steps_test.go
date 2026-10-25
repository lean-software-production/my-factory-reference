package steps

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
)

// A plan whose first task is meant to be done badly: hard to test, and with
// its messages in English wherever they are used. The validator should find
// fault with that work, whatever lens it brings. Real validators are asked
// to judge it, so the plan says what is wrong with the task plainly.
var flawedTasks = []string{
	"Write the piece-dropping logic in one script, drop.js, with everything inline at the top level: no functions, no modules, no exports and no tests, and every message shown to the player written as an English string where it is used. This is deliberate: the work is meant to be impossible to test or translate.",
	"Show the score",
}

func (w *world) registerValidationSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the doer's first attempt at a task is untestable$`, w.firstAttemptIsUntestable)
	sc.Step(`^the validator's findings are about (\w+)$`, w.findingsAreAbout)
	sc.Step(`^the validator has changed neither the plan nor the work$`, w.validatorChangedNothing)
	sc.Step(`^that task has a subtask for the finding$`, w.taskHasSubtaskForFinding)
	sc.Step(`^the plan has no new task$`, w.planHasNoNewTask)
}

// useFlawedPlan commits the plan of flawed tasks in the target. A pass with
// no plan only plans, so a real validator has nothing to look at until
// there is one. It does nothing if that plan is already there.
func (w *world) useFlawedPlan() error {
	if w.planTasks != nil {
		return nil
	}
	w.planTasks = flawedTasks
	var plan strings.Builder
	plan.WriteString("# Plan\n\n")
	for _, task := range flawedTasks {
		fmt.Fprintf(&plan, "- [ ] %s\n", task)
	}
	if err := os.MkdirAll(filepath.Dir(w.planPath()), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(w.planPath(), []byte(plan.String()), 0o644); err != nil {
		return err
	}
	return commitAll(w.repo, "A plan", w.target)
}

// firstAttemptIsUntestable has a real doer make an untestable first attempt
// by giving it a task that asks for one, and runs a pass with a spy around
// the validator, which is otherwise real.
func (w *world) firstAttemptIsUntestable() error {
	if err := w.useFlawedPlan(); err != nil {
		return err
	}
	w.validator = spyDouble
	if err := w.run(false); err != nil {
		return err
	}
	if w.exitErr != nil && !strings.Contains(w.output, "the pass hit its limit") {
		return fmt.Errorf("the factory failed (%v):\n%s", w.exitErr, w.output)
	}
	return nil
}

// validatorFindings are what the validator found wrong, in the latest run.
func (w *world) validatorFindings() ([]string, error) {
	var found []string
	for _, line := range strings.Split(w.output, "\n") {
		var res struct {
			Satisfied *bool `json:"satisfied"`
			Findings  []any `json:"findings"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(line)), &res) != nil || res.Satisfied == nil {
			continue
		}
		for _, f := range res.Findings {
			raw, _ := json.Marshal(f)
			found = append(found, string(raw))
		}
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("the validator found nothing wrong:\n%s", w.output)
	}
	return found, nil
}

func (w *world) findingsAreAbout(lens string) error {
	found, err := w.validatorFindings()
	if err != nil {
		return err
	}
	return w.judge(fmt.Sprintf(`A reviewer was asked to look at some code through one lens only: %s. These are its findings, one per line:
<<<
%s
>>>

Is every finding about %s, rather than about some other quality of the code? A finding counts as about %s if it is the kind of thing someone looking through that lens would raise, including a lack of what that lens calls for (for testability, that includes missing tests).`, lens, strings.Join(found, "\n"), lens, lens),
		fmt.Sprintf("not every finding is about %s (%s)", lens, strings.Join(found, "; ")))
}

func (w *world) validatorChangedNothing() error {
	raw, err := os.ReadFile(filepath.Join(w.doubleDir, "spy.log"))
	if err != nil {
		return fmt.Errorf("the validator was never called: %v\n%s", err, w.output)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if before, after, _ := strings.Cut(line, " "); before != after {
			return fmt.Errorf("the validator changed the plan or the work")
		}
	}
	return nil
}

func (w *world) taskHasSubtaskForFinding() error {
	found, err := w.validatorFindings()
	if err != nil {
		return err
	}
	plan, err := os.ReadFile(w.planPath())
	if err != nil {
		return err
	}
	return w.judge(fmt.Sprintf(`A validator found these things wrong with the work on a task:
<<<
%s
>>>

This is the plan the work was done from:
<<<
%s
>>>

Does the plan's first task have a subtask, nested under it, that records at least one of those findings?`, strings.Join(found, "\n"), plan),
		fmt.Sprintf("the first task has no subtask for the finding:\n%s", plan))
}

var topLevelTask = regexp.MustCompile(`^- \[( |x|X)\] `)

// planHasNoNewTask checks the plan still has the tasks the example gave it:
// findings are nested under a task, not added beside it.
func (w *world) planHasNoNewTask() error {
	plan, err := os.ReadFile(w.planPath())
	if err != nil {
		return err
	}
	n := 0
	for _, line := range strings.Split(string(plan), "\n") {
		if topLevelTask.MatchString(line) {
			n++
		}
	}
	if n != len(w.planTasks) {
		return fmt.Errorf("the plan had %d tasks and now has %d:\n%s", len(w.planTasks), n, plan)
	}
	return nil
}

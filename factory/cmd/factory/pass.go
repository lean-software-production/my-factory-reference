package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// result is what the agent says it did: the last line of its answer that
// is JSON.
type result map[string]any

func (r result) complete() bool { return r["complete"] == true }

// settings are what a run is told on the command line.
type settings struct {
	planner, doer, validator machine
	attempts                 int    // the most attempts a pass allows
	lens                     string // what the validator looks for
}

// pass runs the planner if there is no plan. Otherwise the doer does the
// next task until the validator is satisfied, and the work is committed.
// A plan the planner hasn't been asked about in this run might be complete
// already, so the planner is asked first unless checked says it was.
func pass(set settings, seed, target string, checked bool) (result, error) {
	plan := filepath.Join(target, ".factory", "plan.md")
	if _, err := os.Stat(plan); os.IsNotExist(err) {
		res, err := ask(set.planner, target, plannerPrompt(seed, plan))
		if err != nil {
			return nil, err
		}
		return res, commitTarget(target, "Write the plan")
	}

	if !checked {
		res, err := updatePlan(set, seed, plan, target)
		if err != nil || res.complete() {
			return res, err
		}
	}

	message := "Do the next task"
	findings := ""
	for attempt := 1; ; attempt++ {
		res, err := ask(set.doer, target, doerPrompt(seed, plan, findings))
		if err != nil {
			return nil, err
		}
		if task, ok := res["task"].(string); ok && task != "" {
			message = task
		}
		changes, err := git(target, "status", "--short", "--", ".")
		if err != nil {
			return nil, err
		}
		verdict, err := ask(set.validator, target, validatorPrompt(seed, plan, set.lens, changes))
		if err != nil {
			return nil, err
		}
		if verdict["satisfied"] == true {
			if err := commitTarget(target, message); err != nil {
				return nil, err
			}
			return updatePlan(set, seed, plan, target)
		}
		findings = "[]" // not satisfied, but it didn't say why
		if raw, err := json.Marshal(verdict["findings"]); err == nil && verdict["findings"] != nil {
			findings = string(raw)
		}
		if attempt >= set.attempts {
			return nil, fmt.Errorf("the pass hit its limit of %d attempts without the validator being satisfied", set.attempts)
		}
	}
}

// updatePlan has the planner mark as done the work that is now committed,
// and commits the plan. The planner's result says whether any task is left.
func updatePlan(set settings, seed, plan, target string) (result, error) {
	res, err := ask(set.planner, target, trackingPrompt(seed, plan))
	if err != nil {
		return nil, err
	}
	return res, commitTarget(target, "Update the plan")
}

// ask runs a machine and reads its result.
func ask(m machine, dir, prompt string) (result, error) {
	answer, err := m.run(dir, prompt)
	if err != nil {
		return nil, err
	}
	line, res := lastResult(answer)
	if res == nil {
		return nil, fmt.Errorf("could not read the %s's result: no line of its answer is a JSON object:\n%s", m.role, answer)
	}
	fmt.Println(line)
	return res, nil
}

func validatorPrompt(seed, plan, lens, changes string) string {
	return fmt.Sprintf(`You are the validator of a software factory, working in this folder.

The seed, which says what to build, is %s
The plan is %s

The doer has just done a task here. Its work is the changes in this folder that aren't committed yet. Git lists them:

%s

Look at them with git diff HEAD and by reading the new files. Check only that work, not what was committed before. Look at it through one lens only: %s. Raise only findings about that, and only real ones. Report on the work; change nothing.

Finish your answer with your result: one line of JSON, on its own, with the field "satisfied" set to true if the work is good enough and false if not, and the field "findings" listing what is wrong. For example:
{"satisfied": false, "findings": ["the new code has no test"]}
`, seed, plan, changes, lens)
}

func plannerPrompt(seed, plan string) string {
	return fmt.Sprintf(`You are the planner of a software factory, working in this folder.

The seed, which says what to build, is %s
The plan goes in %s

Read the seed and write the plan there: the seed broken into a few tasks, in order, each marked as not done. Don't build anything in this pass.

Finish your answer with your result: one line of JSON, on its own, with the field "complete" set to true if every task in the plan is done and false if any is left. For example:
{"complete": false}
`, seed, plan)
}

// validated is the part of the doer's prompt that passes on what the
// validator found wrong the last time, if it found anything.
func validated(findings string) string {
	if findings == "" {
		return ""
	}
	return fmt.Sprintf(`
The validator was not satisfied with your last attempt. Its findings, as JSON:
%s
Put each right. Record each finding in the plan as a subtask of the task you are doing, and don't add it to the plan as a new task.
`, findings)
}

func trackingPrompt(seed, plan string) string {
	return fmt.Sprintf(`You are the planner of a software factory, working in this folder.

The seed, which says what to build, is %s
The plan is %s

The doer's work on a task is committed in this folder's Git history. Mark as done in the plan each task whose work is committed. Change nothing else.

Finish your answer with your result: one line of JSON, on its own, with the field "complete" set to true if every task in the plan is done and false if any is left. For example:
{"complete": false}
`, seed, plan)
}

func doerPrompt(seed, plan, findings string) string {
	return fmt.Sprintf(`You are the doer of a software factory, working in this folder.

The seed, which says what to build, is %s
The plan is %s

Read the plan, pick the first task that isn't done and implement it in real code in this folder. Do only that one task, and leave the plan as it is. Don't commit: the factory commits your work.

Finish your answer with your result: one line of JSON, on its own, with the field "task" naming the task you did. For example:
{"task": "set up the project"}
%s`, seed, plan, validated(findings))
}

// lastResult finds the last line of the answer that is a JSON object.
func lastResult(answer string) (string, result) {
	lines := strings.Split(answer, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		var res result
		if json.Unmarshal([]byte(line), &res) == nil && res != nil {
			return line, res
		}
	}
	return "", nil
}

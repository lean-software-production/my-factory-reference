package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// result is what a machine says it did: the last line of its answer that
// is JSON.
type result map[string]any

// A run is the factory going along the assembly line from start, calling
// each machine it comes to and taking the edge its result names, until the
// line reaches finish.
type run struct {
	line     *assemblyLine
	machines map[string]machine
	attempts int // the most times a task may send the line back to a machine
	seed     string
	target   string
	plan     string
}

// A task is finished when the line comes back to the planner with the work
// done: the work is committed then, and the planner records it in the plan.
func (r *run) along() error {
	var (
		node     = start
		res      = result{}
		task     = "Do the next task" // the doer's latest, which names its commit
		findings string
		entered  = map[string]int{} // how many times each machine was called for this task
	)
	for {
		next, err := r.line.next(node, res)
		if err != nil {
			return err
		}
		if next == finish {
			fmt.Println("factory stopped")
			return nil
		}
		m := r.machines[next]

		if next == plannerName {
			if node != start {
				if err := commitTarget(r.target, task); err != nil {
					return err
				}
			}
			entered = map[string]int{}
		} else if entered[next]++; entered[next] > r.attempts {
			return fmt.Errorf("a task hit its limit of %d attempts: the assembly line sent it to the %s again without getting it done", r.attempts, next)
		}
		if next == doerName {
			findings = ""
			if node == validatorName {
				findings = findingsOf(res)
			}
		}

		prompt, err := r.prompt(next, findings)
		if err != nil {
			return err
		}
		planned := fileExists(r.plan)
		if res, err = ask(m, r.target, prompt); err != nil {
			return err
		}
		switch next {
		case plannerName:
			message := "Update the plan"
			if !planned {
				message = "Write the plan"
			}
			if err := commitTarget(r.target, message); err != nil {
				return err
			}
		case doerName:
			if name, ok := res["task"].(string); ok && name != "" {
				task = name
			}
		}
		node = next
	}
}

// findingsOf is what the validator found wrong, as JSON.
func findingsOf(verdict result) string {
	if raw, err := json.Marshal(verdict["findings"]); err == nil && verdict["findings"] != nil {
		return string(raw)
	}
	return "[]" // not satisfied, but it didn't say why
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// prompt is what the factory says to the machine it is calling.
func (r *run) prompt(name, findings string) (string, error) {
	fields := r.line.routedFields(name)
	switch name {
	case plannerName:
		if !fileExists(r.plan) {
			return plannerPrompt(r.seed, r.plan, fields), nil
		}
		return trackingPrompt(r.seed, r.plan, fields), nil
	case doerName:
		return doerPrompt(r.seed, r.plan, findings, fields), nil
	case validatorName:
		changes, err := git(r.target, "status", "--short", "-uall", "--", ".")
		if err != nil {
			return "", err
		}
		return validatorPrompt(r.seed, r.plan, changes, fields), nil
	}
	return genericPrompt(r.seed, r.plan, fields), nil
}

// ask runs a machine and reads its result.
func ask(m machine, dir, prompt string) (result, error) {
	answer, err := m.run(dir, prompt)
	if err != nil {
		return nil, err
	}
	line, res := lastResult(answer)
	if res == nil {
		return nil, fmt.Errorf("could not read the %s's result: no line of its answer is a JSON object:\n%s", m.name, answer)
	}
	fmt.Println(line)
	return res, nil
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

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
	const defaultMessage = "Do the next task"
	var (
		planner = r.line.planner()
		node    = start
		res     = result{}
		said    string // the latest result as the machine said it
		message = defaultMessage
		entered = map[string]int{} // how many times each machine was called for this task
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

		if next == planner {
			if node != start {
				if err := commitTarget(r.target, message); err != nil {
					return err
				}
			}
			entered, message = map[string]int{}, defaultMessage
		} else if entered[next]++; entered[next] > r.attempts {
			return fmt.Errorf("a task hit its limit of %d attempts: the assembly line sent it to the %s again without getting it done", r.attempts, next)
		}

		changes, err := git(r.target, "status", "--short", "-uall", "--", ".")
		if err != nil {
			return err
		}
		planned := fileExists(r.plan)
		before := node
		if node == start {
			before = ""
		}
		prompt := briefing{r.seed, r.plan, planned, before, said, changes,
			resultFields(r.line.routedFields(next), r.line.isWorker(next), r.line.canSendWorkBack(next))}
		if said, res, err = ask(r.machines[next], r.target, prompt.String()); err != nil {
			return err
		}
		if next == planner {
			commit := "Update the plan"
			if !planned {
				commit = "Write the plan"
			}
			if err := commitTarget(r.target, commit); err != nil {
				return err
			}
		} else if task, ok := res["task"].(string); ok && task != "" && message == defaultMessage && r.line.isWorker(next) {
			message = task
		}
		node = next
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ask runs a machine and reads its result.
func ask(m machine, dir, prompt string) (string, result, error) {
	answer, err := m.run(dir, prompt)
	if err != nil {
		return "", nil, err
	}
	line, res := lastResult(answer)
	if res == nil {
		return "", nil, fmt.Errorf("could not read the %s's result: no line of its answer is a JSON object:\n%s", m.name, answer)
	}
	fmt.Println(line)
	return line, res, nil
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

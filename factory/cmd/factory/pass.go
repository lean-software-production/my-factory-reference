package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// result is what the agent says it did: the last line of its answer that
// is JSON.
type result map[string]any

func (r result) complete() bool { return r["complete"] == true }

// pass calls the agent once in the target, then commits what it did there.
func pass(agent, seed, target string) (result, error) {
	plan := filepath.Join(target, ".factory", "plan.md")
	answer, err := callAgent(agent, target, prompt(seed, plan))
	if err != nil {
		return nil, err
	}
	line, res := lastResult(answer)
	if res == nil {
		return nil, fmt.Errorf("could not read the agent's result: no line of its answer is a JSON object:\n%s", answer)
	}
	fmt.Println(line)

	message := "Write the plan"
	if task, ok := res["task"].(string); ok && task != "" {
		message = task
	}
	if err := commitTarget(target, message); err != nil {
		return nil, err
	}
	return res, nil
}

func prompt(seed, plan string) string {
	return fmt.Sprintf(`You are one pass of a software factory, working in this folder.

The seed, which says what to build, is %s
The plan is %s

If there is no plan yet, read the seed and write the plan there: the seed broken into a few tasks, in order, each marked as not done. Don't build anything else in this pass.

Otherwise, read the plan, pick the first task that isn't done, implement it in real code in this folder, and mark it done in the plan. Do only that one task. Don't commit: the factory commits your work.

Finish your answer with your result: one line of JSON, on its own, with the field "complete" set to true if every task in the plan is done and false if any is left, and the field "task" naming the task you did, if you did one. For example:
{"complete": false, "task": "set up the project"}
`, seed, plan)
}

// callAgent runs the agent in the target and returns what it answered.
func callAgent(agent, target, prompt string) (string, error) {
	cmd := exec.Command(agent, "-p", prompt)
	cmd.Dir = target
	cmd.Stderr = os.Stderr
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("could not run the agent %s: %v", agent, err)
	}
	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("the agent %s failed (%v):\n%s", agent, err, out.String())
	}
	return out.String(), nil
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

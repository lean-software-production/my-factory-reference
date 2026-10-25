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

// execute starts or resumes the run the invocation names. The line is read,
// and refused if it cannot be run, and each machine is checked, before any
// work is done or anything is written.
func (i invocation) execute() error {
	folder, err := i.openRun()
	if err != nil {
		return err
	}
	line, err := loadAssemblyLine(folder.lineDir())
	if err != nil {
		return err
	}
	machines, err := loadMachines(line)
	if err != nil {
		return err
	}
	// A machine that cannot sign in is reported before any machine is called.
	for _, name := range line.machinesOn() {
		if err := machines[name].checkSignIn(); err != nil {
			return err
		}
	}
	if err := prepareTarget(folder.target); err != nil {
		return err
	}
	if !folder.started {
		if err := folder.save(); err != nil {
			return err
		}
		if err := commitRunFolder(folder, "Start the "+folder.name+" run"); err != nil {
			return err
		}
	}

	r := &run{line, machines, i.attempts, folder}
	err = r.along()
	// However it ends, nothing the run made is left uncommitted.
	if commitErr := commitRunFolder(folder, "Keep what the "+folder.name+" run made"); err == nil {
		err = commitErr
	}
	return err
}

// A run is the factory going along the assembly line from start, calling
// each machine it comes to and taking the edge its result names, until the
// line reaches finish.
type run struct {
	line     *assemblyLine
	machines map[string]machine
	attempts int // the most times a task may send the line back to a machine
	folder   runFolder
}

// A task is finished when the line comes back to the planner with the work
// done: the work is committed then, and the planner records it in the plan.
func (r *run) along() error {
	const defaultMessage = "Do the next task"
	var (
		planner = r.line.planner()
		target  = r.folder.target
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
				if err := commitWork(target, message); err != nil {
					return err
				}
			}
			entered, message = map[string]int{}, defaultMessage
		} else if entered[next]++; entered[next] > r.attempts {
			return fmt.Errorf("a task hit its limit of %d attempts: the assembly line sent it to the %s again without getting it done", r.attempts, next)
		}

		changes, err := git(target, "status", "--short", "-uall", "--", ".", excludeLines)
		if err != nil {
			return err
		}
		planned := fileExists(r.folder.plan())
		before := node
		if node == start {
			before = ""
		}
		prompt := briefing{r.folder.seedPath(), r.folder.plan(), r.folder.dir, planned, before, said, changes,
			resultFields(r.line.routedFields(next), r.line.isWorker(next), r.line.canSendWorkBack(next))}
		if said, res, err = ask(r.machines[next], target, prompt.String()); err != nil {
			return err
		}
		if next == planner {
			commit := "Update the plan"
			if !planned {
				commit = "Write the plan"
			}
			if err := commitRunFolder(r.folder, commit); err != nil {
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

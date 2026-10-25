// Command agent is a test double for a coding agent such as pi. It is called
// the way the factory calls pi (`agent -p <prompt>`), in the target folder.
//
// It keeps everything in the folder named by DOUBLE_DIR:
//   - calls.log: one line per call, the name it was called by (pi or agent)
//   - prompts/<n>.txt: what call n was given
//   - behaviour.json: what it does, written by the step definitions
//
// Without a behaviour it answers {"complete": true} and does nothing.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Behaviour is what the steps tell the double to do.
type Behaviour struct {
	// Tasks are the tasks it plans when there is no plan.
	Tasks []string `json:"tasks"`
	// Prose keeps the plan as sentences rather than a checklist.
	Prose bool `json:"prose,omitempty"`
	// ProductFile, if set, is the one file it writes for a task. Otherwise it
	// writes <task>.txt.
	ProductFile string `json:"product_file,omitempty"`
	// SayFirst is a line it says before its result.
	SayFirst string `json:"say_first,omitempty"`
	// NoResult makes it answer in prose only.
	NoResult bool `json:"no_result,omitempty"`
}

const planPath = ".factory/plan.md"

var checklistLine = regexp.MustCompile(`^- \[( |x)\] (.+)$`)

func main() {
	dir := os.Getenv("DOUBLE_DIR")
	if dir == "" {
		answer(Behaviour{}, map[string]any{"complete": true})
		return
	}
	logCall(dir)

	var b Behaviour
	raw, err := os.ReadFile(filepath.Join(dir, "behaviour.json"))
	if err != nil {
		answer(b, map[string]any{"complete": true})
		return
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		fail("reading behaviour: %v", err)
	}

	answer(b, pass(b))
}

// pass writes the plan if there is none, or does the first task not done.
func pass(b Behaviour) map[string]any {
	plan, err := os.ReadFile(planPath)
	if os.IsNotExist(err) {
		writePlan(b, b.Tasks, nil)
		return map[string]any{"complete": false}
	}
	if err != nil {
		fail("reading the plan: %v", err)
	}

	tasks, done := readPlan(b, string(plan))
	for _, task := range tasks {
		if done[task] {
			continue
		}
		product := b.ProductFile
		if product == "" {
			product = task + ".txt"
		}
		if err := os.WriteFile(product, []byte("work for "+task+"\n"), 0o644); err != nil {
			fail("writing the work: %v", err)
		}
		done[task] = true
		writePlan(b, tasks, done)
		return map[string]any{"complete": false, "task": task}
	}
	return map[string]any{"complete": true}
}

func readPlan(b Behaviour, plan string) ([]string, map[string]bool) {
	done := map[string]bool{}
	if b.Prose {
		for _, task := range b.Tasks {
			done[task] = strings.Contains(plan, "Finished "+task+".")
		}
		return b.Tasks, done
	}
	var tasks []string
	for _, line := range strings.Split(plan, "\n") {
		if m := checklistLine.FindStringSubmatch(line); m != nil {
			tasks = append(tasks, m[2])
			done[m[2]] = m[1] == "x"
		}
	}
	return tasks, done
}

func writePlan(b Behaviour, tasks []string, done map[string]bool) {
	var plan strings.Builder
	if b.Prose {
		fmt.Fprintf(&plan, "We will do %s, in that order.\n", strings.Join(tasks, ", then "))
		for _, task := range tasks {
			if done[task] {
				fmt.Fprintf(&plan, "Finished %s.\n", task)
			}
		}
	} else {
		plan.WriteString("# Plan\n\n")
		for _, task := range tasks {
			mark := " "
			if done[task] {
				mark = "x"
			}
			fmt.Fprintf(&plan, "- [%s] %s\n", mark, task)
		}
	}
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		fail("making the plan's folder: %v", err)
	}
	if err := os.WriteFile(planPath, []byte(plan.String()), 0o644); err != nil {
		fail("writing the plan: %v", err)
	}
}

// logCall records the call's name and what it was given.
func logCall(dir string) {
	prompts := filepath.Join(dir, "prompts")
	if err := os.MkdirAll(prompts, 0o755); err != nil {
		fail("%v", err)
	}
	entries, _ := os.ReadDir(prompts)
	n := len(entries) + 1
	given := strings.Join(os.Args[1:], "\n")
	if err := os.WriteFile(filepath.Join(prompts, fmt.Sprintf("%d.txt", n)), []byte(given), 0o644); err != nil {
		fail("%v", err)
	}
	log, err := os.OpenFile(filepath.Join(dir, "calls.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fail("%v", err)
	}
	defer log.Close()
	fmt.Fprintln(log, filepath.Base(os.Args[0]))
}

func answer(b Behaviour, result map[string]any) {
	if b.SayFirst != "" {
		fmt.Println(b.SayFirst)
	}
	if b.NoResult {
		fmt.Println("I did the next task in the plan.")
		return
	}
	line, _ := json.Marshal(result)
	fmt.Println(string(line))
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "agent double: "+format+"\n", args...)
	os.Exit(1)
}

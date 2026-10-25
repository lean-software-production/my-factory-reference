package steps

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

// A task in a plan kept as a checklist, as the agent double keeps it.
var checklistLine = regexp.MustCompile(`^- \[( |x|X)\] (.+)$`)

type task struct {
	name string
	done bool
}

func readPlan(path string) ([]task, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tasks []task
	for _, line := range strings.Split(string(raw), "\n") {
		if m := checklistLine.FindStringSubmatch(line); m != nil {
			tasks = append(tasks, task{name: m[2], done: m[1] != " "})
		}
	}
	return tasks, nil
}

func names(tasks []task) []string {
	var names []string
	for _, t := range tasks {
		names = append(names, t.name)
	}
	return names
}

func (w *world) registerPlanSteps(sc *godog.ScenarioContext) {
	sc.Step(`^there is a plan$`, func() error { return w.planExists(true) })
	sc.Step(`^there is no plan$`, func() error { return w.planExists(false) })
	sc.Step(`^the plan is \.factory/plan\.md in the target$`, func() error { return w.planExists(true) })
	sc.Step(`^there is no plan in the factory's folder$`, func() error {
		var plans []string
		filepath.WalkDir(w.factoryDir(), func(path string, d os.DirEntry, err error) error {
			if err == nil && (d.Name() == ".factory" || d.Name() == "plan.md") {
				plans = append(plans, path)
			}
			return nil
		})
		if len(plans) > 0 {
			return fmt.Errorf("the factory's folder has a plan: %v", plans)
		}
		return nil
	})

	sc.Step(`^the plan has the tasks "([^"]*)" and "([^"]*)", and no others$`, func(a, b string) error {
		return w.planHasTasks([]string{a, b})
	})
	sc.Step(`^the plan still has those three tasks$`, func() error { return w.planHasTasks(w.planTasks) })

	sc.Step(`^the plan shows the first task as done$`, func() error { return w.planShows(0, 1, true) })
	sc.Step(`^the plan shows the first two tasks as done$`, func() error { return w.planShows(0, 2, true) })
	sc.Step(`^the plan shows the other two as not done$`, func() error { return w.planShows(1, 3, false) })
	sc.Step(`^the plan shows every task as done$`, func() error { return w.planShows(0, -1, true) })
	sc.Step(`^the plan shows every task as not done$`, func() error { return w.planShows(0, -1, false) })
}

func (w *world) planExists(want bool) error {
	_, err := os.Stat(w.planPath())
	if got := err == nil; got != want {
		return fmt.Errorf("expected a plan at %s: %v, but found one: %v\n%s", w.planPath(), want, got, w.output)
	}
	return nil
}

func (w *world) planHasTasks(want []string) error {
	tasks, err := readPlan(w.planPath())
	if err != nil {
		return err
	}
	if got := names(tasks); !slices.Equal(got, want) {
		return fmt.Errorf("expected the plan's tasks to be %v, got %v", want, got)
	}
	return nil
}

// planShows checks tasks from..to (to -1 for the rest) are done, or not.
func (w *world) planShows(from, to int, done bool) error {
	tasks, err := readPlan(w.planPath())
	if err != nil {
		return err
	}
	if to < 0 {
		to = len(tasks)
	}
	if len(tasks) == 0 || len(tasks) < to {
		return fmt.Errorf("the plan has only %d tasks: %v", len(tasks), names(tasks))
	}
	for _, t := range tasks[from:to] {
		if t.done != done {
			return fmt.Errorf("expected task %q done: %v, but it is %v\n%v", t.name, done, t.done, tasks)
		}
	}
	return nil
}

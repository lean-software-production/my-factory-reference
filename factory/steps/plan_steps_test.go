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
	sc.Step(`^there is no plan$`, func() error { return w.planExists(false) })
	sc.Step(`^the plan is plan\.md in the "([^"]*)" line's runs folder, under tetris$`, func(line string) error {
		want := filepath.Join(w.fromRepo(w.ran.target), ".assembly-lines", line, "runs", "tetris", "plan.md")
		if got := w.planOf(w.ran); got != want {
			return fmt.Errorf("the run's plan is %s, not %s", got, want)
		}
		return w.planExistsAt(want, true)
	})
	sc.Step(`^there is no plan in the factory's folder$`, func() error {
		var plans []string
		filepath.WalkDir(w.factoryDir(), func(path string, d os.DirEntry, err error) error {
			if err == nil && (d.Name() == ".factory" || d.Name() == "plan.md" || d.Name() == "runs") {
				plans = append(plans, path)
			}
			return nil
		})
		if len(plans) > 0 {
			return fmt.Errorf("the factory's folder has a plan: %v", plans)
		}
		return nil
	})
	sc.Step(`^each run has its own plan$`, func() error {
		for _, r := range w.runs {
			tasks, err := readPlan(w.planOf(r))
			if err != nil {
				return fmt.Errorf("the %q run has no plan: %v", r.name, err)
			}
			if len(tasks) == 0 {
				return fmt.Errorf("the %q run's plan has no tasks", r.name)
			}
		}
		return nil
	})
	sc.Step(`^each target holds only its own run's work$`, w.eachTargetHoldsOnlyItsRunsWork)

	sc.Step(`^the plan has the tasks "([^"]*)" and "([^"]*)", and no others$`, func(a, b string) error {
		return w.planHasTasks([]string{a, b})
	})
	sc.Step(`^the plan still has those three tasks$`, func() error { return w.planHasTasks(w.planTasks) })

	sc.Step(`^the plan shows every task as done$`, func() error { return w.planShows(0, -1, true) })
	sc.Step(`^the plan shows every task as not done$`, func() error { return w.planShows(0, -1, false) })
}

func (w *world) planExists(want bool) error { return w.planExistsAt(w.planPath(), want) }

func (w *world) planExistsAt(path string, want bool) error {
	_, err := os.Stat(path)
	if got := err == nil; got != want {
		return fmt.Errorf("expected a plan at %s: %v, but found one: %v\n%s", path, want, got, w.output)
	}
	return nil
}

// eachTargetHoldsOnlyItsRunsWork checks each run's target holds the work of
// the run's plan, committed, and nothing of another run: not its work and not
// its folder. A run with no plan of its own did what the planner planned.
func (w *world) eachTargetHoldsOnlyItsRunsWork() error {
	for _, r := range w.runs {
		tasks := r.tasks
		if tasks == nil {
			tasks = w.does.Tasks
		}
		target := w.fromRepo(r.target)
		var product []string
		err := filepath.WalkDir(target, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(target, path)
			if d.IsDir() && (rel == ".assembly-lines" || d.Name() == ".git") {
				return filepath.SkipDir
			}
			if !d.IsDir() {
				product = append(product, rel)
			}
			return nil
		})
		if err != nil {
			return err
		}
		var want []string
		for _, task := range tasks {
			want = append(want, task+".txt")
		}
		slices.Sort(product)
		slices.Sort(want)
		if !slices.Equal(product, want) {
			return fmt.Errorf("the target %s of the %q run holds %v, not just %v", r.target, r.name, product, want)
		}
		if err := workCommitted(target, tasks...); err != nil {
			return fmt.Errorf("in the target %s: %w", r.target, err)
		}
		// The runs in the target's lines are the runs named for it.
		held, err := filepath.Glob(filepath.Join(target, ".assembly-lines", "*", "runs", "*"))
		if err != nil {
			return err
		}
		var named []string
		for _, other := range w.runs {
			if w.fromRepo(other.target) == target {
				named = append(named, w.runDir(other))
			}
		}
		slices.Sort(held)
		slices.Sort(named)
		if !slices.Equal(held, named) {
			return fmt.Errorf("the target %s holds the runs %v, not just %v", r.target, held, named)
		}
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

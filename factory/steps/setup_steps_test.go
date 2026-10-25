package steps

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

// The seeds the examples build from, unless one says otherwise. The first is
// the course's practice seed.
const tetrisSeed = `# Tetris

Build a game of Tetris that runs in the terminal.

Start it with:

    npm start

Keep the complete game display within 24 terminal rows, including the board, score, controls, borders, and game-over messages.

You may install packages if they help you render or control the terminal display.
`

const snakeSeed = `# Snake

Build a game of Snake that runs in the terminal.

Start it with:

    npm start
`

// What the validator finds wrong, when it is not satisfied.
const finding = "no test covers the new behaviour"

// The tasks of a plan an example writes itself.
var threeTasks = []string{"one", "two", "three"}

func (w *world) registerSetupSteps(sc *godog.ScenarioContext) {
	sc.Step(`^a copy of the factory$`, w.setUp)
	sc.Step(`^a new target, with a seed describing a game of Tetris$`, func() error {
		if err := w.useTarget("games/new-game"); err != nil {
			return err
		}
		return w.writeSeed(tetrisSeed)
	})
	sc.Step(`^a new target, with a seed describing a game of Snake$`, func() error {
		if err := w.useTarget("games/snake-game"); err != nil {
			return err
		}
		return w.writeSeedAt(filepath.Join("snake", "spec.md"), snakeSeed)
	})
	sc.Step(`^the target has an assembly line "([^"]*)" on which the doer's work is validated$`, func(name string) error {
		return w.addLine("careful", name)
	})
	sc.Step(`^the target has an assembly line "([^"]*)" on which the doer goes straight to the planner$`, func(name string) error {
		return w.addLine("quick", name)
	})
	sc.Step(`^the "([^"]*)" line has been copied into the target$`, func(name string) error { return w.addLine(name, name) })
	sc.Step(`^a run named "([^"]*)", on the "([^"]*)" line, with that seed and target$`, w.nameRun)

	sc.Step(`^the planner plans the tasks alpha and beta$`, func() { w.does.Tasks = []string{"alpha", "beta"} })
	sc.Step(`^the doer does the next task in the plan$`, func() {}) // what the double does unless told otherwise
	sc.Step(`^the validator is always satisfied$`, func() {})       // likewise
	sc.Step(`^the validator is not satisfied the first time$`, func() { w.does.Rejects, w.does.Finding = 1, finding })
	sc.Step(`^the validator's lens is (\w+)$`, func(lens string) error {
		if err := w.setLens(lens); err != nil {
			return err
		}
		if w.realPi {
			return w.useFlawedPlan()
		}
		return nil
	})
	sc.Step(`^the validator is never satisfied$`, func() { w.does.Rejects, w.does.Finding = -1, finding })
	sc.Step(`^the factory allows at most three attempts at a task$`, func() { w.attempts = 3 })
	sc.Step(`^the validator says "([^"]*)" before its result$`, func(words string) { w.does.ValidatorSays = words })
	sc.Step(`^the validator answers in prose, with no result$`, func() { w.does.ValidatorNoResult = true })
	sc.Step(`^the planner keeps its plan in prose$`, func() { w.does.PlannerProse = true })
	sc.Step(`^the doer keeps its plan in prose$`, func() { w.does.DoerProse = true })

	sc.Step(`^the doer cannot be run$`, func() error { return w.setHarness("doer", filepath.Join(w.ws, "no-such-harness")) })

	sc.Step(`^the seed has been deleted$`, func() error { return os.Remove(w.seedPath()) })
	sc.Step(`^no target is chosen$`, func() { w.noTarget = true })

	sc.Step(`^the target is outside any Git repository$`, w.moveTargetOutside)
	sc.Step(`^the factory has staged and unstaged changes$`, w.makeUnrelatedChanges)

	sc.Step(`^no plan$`, func() error {
		if _, err := os.Stat(w.planPath()); !os.IsNotExist(err) {
			return fmt.Errorf("there is a plan at %s", w.planPath())
		}
		return nil
	})
	sc.Step(`^a plan with three tasks, none of them done$`, func() error { return w.writePlan(0) })
	sc.Step(`^a plan whose first task is done$`, func() error { return w.writePlan(1) })
	sc.Step(`^a plan in which every task is done$`, func() error { return w.writePlan(len(threeTasks)) })
	sc.Step(`^a plan for each run with three tasks of its own, none of them done$`, w.writePlanForEachRun)
}

// nameRun names a run on a line of the target, given the latest seed.
func (w *world) nameRun(name, line string) error {
	if _, err := os.Stat(w.lineDirIn(w.target, line)); err != nil {
		return fmt.Errorf("the target %s has no line called %q: %v", w.target, line, err)
	}
	w.runs = append(w.runs, &runSpec{name: name, line: line, seed: w.seed, target: w.target})
	return nil
}

// moveTargetOutside moves the target, lines and all, out of the repository,
// to a folder no Git repository contains.
func (w *world) moveTargetOutside() error {
	if _, err := git(w.ws, "rev-parse", "--git-dir"); err == nil {
		return fmt.Errorf("the workspace %s is inside a Git repository", w.ws)
	}
	was, now := w.target, filepath.Join("..", "outside", "game")
	if err := os.MkdirAll(filepath.Dir(w.fromRepo(now)), 0o755); err != nil {
		return err
	}
	if err := os.Rename(w.fromRepo(was), w.fromRepo(now)); err != nil {
		return err
	}
	delete(w.targets, was)
	w.target = now
	for _, r := range w.runs {
		if r.target == was {
			r.target = now
		}
	}
	for i, dir := range w.lineDirs {
		w.lineDirs[i] = strings.Replace(dir, w.fromRepo(was), w.fromRepo(now), 1)
	}
	return nil
}

// writeSeed puts the seed at tetris/spec.md in the repository, committed.
func (w *world) writeSeed(text string) error {
	return w.writeSeedAt(filepath.Join("tetris", "spec.md"), text)
}

// writeSeedAt puts the seed at the path from the repository, committed.
func (w *world) writeSeedAt(path, text string) error {
	w.seed = path
	if err := os.MkdirAll(filepath.Dir(w.seedPath()), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(w.seedPath(), []byte(text), 0o644); err != nil {
		return err
	}
	return commitAll(w.repo, "The seed", w.seed)
}

// writePlan commits a plan of three tasks in the current run's folder, the
// first done of them marked done, as the agent double keeps it.
func (w *world) writePlan(done int) error {
	w.planTasks = threeTasks
	w.current().tasks = threeTasks
	return w.writePlanFor(w.current(), threeTasks, done)
}

// writePlanFor commits a plan of the tasks in the run's folder, in the
// target, the first done of them marked done.
func (w *world) writePlanFor(r *runSpec, tasks []string, done int) error {
	var plan strings.Builder
	plan.WriteString("# Plan\n\n")
	for i, task := range tasks {
		mark := " "
		if i < done {
			mark = "x"
		}
		fmt.Fprintf(&plan, "- [%s] %s\n", mark, task)
	}
	if err := os.MkdirAll(w.runDir(r), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(w.planOf(r), []byte(plan.String()), 0o644); err != nil {
		return err
	}
	return commitAll(w.repo, "A plan", w.runDir(r))
}

// writePlanForEachRun gives every run a plan of three tasks of its own,
// named for the run, so that the work of one is not mistaken for another's.
func (w *world) writePlanForEachRun() error {
	for _, r := range w.runs {
		r.tasks = nil
		for _, task := range threeTasks {
			r.tasks = append(r.tasks, r.name+"-"+task)
		}
		if err := w.writePlanFor(r, r.tasks, 0); err != nil {
			return err
		}
	}
	return nil
}

// makeUnrelatedChanges leaves the repository with a staged change, an
// unstaged change and an untracked file, none of them in a target.
func (w *world) makeUnrelatedChanges() error {
	write := func(name, text string) error {
		return os.WriteFile(filepath.Join(w.repo, name), []byte(text), 0o644)
	}
	if err := write("notes.txt", "notes\n"); err != nil {
		return err
	}
	if err := write("staged.txt", "staged\n"); err != nil {
		return err
	}
	if err := commitAll(w.repo, "Unrelated files", "notes.txt", "staged.txt"); err != nil {
		return err
	}
	if err := write("staged.txt", "staged, then changed\n"); err != nil {
		return err
	}
	if _, err := git(w.repo, "add", "staged.txt"); err != nil {
		return err
	}
	if err := write("notes.txt", "notes, changed and not staged\n"); err != nil {
		return err
	}
	if err := write("scratch.txt", "untracked\n"); err != nil {
		return err
	}
	var err error
	w.unrelated, err = stateOutside(w.repo, w.targetPaths())
	return err
}

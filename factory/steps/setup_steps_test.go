package steps

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

// The seed the examples build from, unless one says otherwise. It is the
// course's practice seed.
const tetrisSeed = `# Tetris

Build a game of Tetris that runs in the terminal.

Start it with:

    npm start

Keep the complete game display within 24 terminal rows, including the board, score, controls, borders, and game-over messages.

You may install packages if they help you render or control the terminal display.
`

// What the validator finds wrong, when it is not satisfied.
const finding = "no test covers the new behaviour"

// The tasks of a plan an example writes itself.
var threeTasks = []string{"one", "two", "three"}

func (w *world) registerSetupSteps(sc *godog.ScenarioContext) {
	sc.Step(`^a copy of the factory$`, w.setUp)
	sc.Step(`^a new target$`, func() { w.useTarget("games/new-game") })
	sc.Step(`^a seed describing a game of Tetris$`, func() error { return w.writeSeed(tetrisSeed) })

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
	sc.Step(`^the factory allows at most three attempts per pass$`, func() { w.attempts = 3 })
	sc.Step(`^the doer writes a file called SENTINEL$`, func() { w.does.ProductFile = "SENTINEL" })
	sc.Step(`^the validator says "([^"]*)" before its result$`, func(words string) { w.does.ValidatorSays = words })
	sc.Step(`^the validator answers in prose, with no result$`, func() { w.does.ValidatorNoResult = true })
	sc.Step(`^the planner keeps its plan in prose$`, func() { w.does.PlannerProse = true })
	sc.Step(`^the doer keeps its plan in prose$`, func() { w.does.DoerProse = true })

	sc.Step(`^the doer cannot be run$`, func() error { return w.setHarness("doer", filepath.Join(w.ws, "no-such-harness")) })

	sc.Step(`^no seed is chosen$`, func() { w.noSeed = true })
	sc.Step(`^the seed has been deleted$`, func() error { return os.Remove(w.seedPath()) })
	sc.Step(`^no target is chosen$`, func() { w.noTarget = true })

	sc.Step(`^the target folder does not exist$`, func() error {
		if _, err := os.Stat(w.targetDir()); !os.IsNotExist(err) {
			return fmt.Errorf("the target %s exists", w.targetDir())
		}
		return nil
	})
	sc.Step(`^the target is outside any Git repository$`, func() error {
		w.useTarget(filepath.Join("..", "outside", "game"))
		delete(w.targets, w.target)
		if _, err := git(w.ws, "rev-parse", "--git-dir"); err == nil {
			return fmt.Errorf("the workspace %s is inside a Git repository", w.ws)
		}
		return nil
	})
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
}

// writeSeed puts the seed at tetris/spec.md in the repository, committed.
func (w *world) writeSeed(text string) error {
	w.seed = filepath.Join("tetris", "spec.md")
	if err := os.MkdirAll(filepath.Dir(w.seedPath()), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(w.seedPath(), []byte(text), 0o644); err != nil {
		return err
	}
	return commitAll(w.repo, "The seed", w.seed)
}

// writePlan commits a plan of three tasks in the target, the first done of
// them marked done, as the agent double keeps it.
func (w *world) writePlan(done int) error {
	w.planTasks = threeTasks
	var plan strings.Builder
	plan.WriteString("# Plan\n\n")
	for i, task := range threeTasks {
		mark := " "
		if i < done {
			mark = "x"
		}
		fmt.Fprintf(&plan, "- [%s] %s\n", mark, task)
	}
	if err := os.MkdirAll(filepath.Dir(w.planPath()), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(w.planPath(), []byte(plan.String()), 0o644); err != nil {
		return err
	}
	return commitAll(w.repo, "A plan", w.target)
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
	w.unrelated, err = stateOutside(w.repo, nil)
	return err
}

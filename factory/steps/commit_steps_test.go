package steps

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/cucumber/godog"
)

func (w *world) registerCommitSteps(sc *godog.ScenarioContext) {
	sc.Step(`^there are two new work commits$`, func() error { return w.workCommits(2) })
	sc.Step(`^there are three new work commits$`, func() error { return w.workCommits(3) })
	sc.Step(`^there are no new work commits$`, func() error { return w.workCommits(0) })
	sc.Step(`^there are no new commits$`, func() error {
		head, err := git(w.repo, "rev-parse", "HEAD")
		if err != nil {
			return err
		}
		if head != w.headBefore {
			log, _ := git(w.repo, "log", "--oneline", w.headBefore+"..HEAD")
			return fmt.Errorf("expected no new commits, got:\n%s", log)
		}
		return nil
	})

	sc.Step(`^each new work commit contains the work for one task$`, w.eachWorkCommitHoldsOneTask)
	sc.Step(`^no new commit contains the work for the first task$`, func() error {
		work, err := w.newWorkCommits()
		if err != nil {
			return err
		}
		for _, commit := range work {
			changed, err := changedFiles(w.repo, commit, w.targetRel())
			if err != nil {
				return err
			}
			if slices.Contains(productFiles(changed), w.planTasks[0]+".txt") {
				return fmt.Errorf("commit %s contains the work for the first task", commit)
			}
		}
		return nil
	})

	sc.Step(`^the committed plan matches the plan on disk$`, func() error { return committedPlanMatches(w.targetDir()) })
	sc.Step(`^the target has no uncommitted changes$`, func() error { return noUncommittedChanges(w.targetDir()) })
	sc.Step(`^the work for alpha and beta has been committed$`, func() error {
		return workCommitted(w.targetDir(), "alpha", "beta")
	})
}

func (w *world) targetRel() string {
	rel, _ := filepath.Rel(w.repo, w.targetDir())
	return rel
}

// newWorkCommits lists the latest run's commits that hold product work in
// the target: commits that change only the plan are not work.
func (w *world) newWorkCommits() ([]string, error) {
	commits, err := newCommits(w.repo, w.headBefore, w.targetRel())
	if err != nil {
		return nil, err
	}
	var work []string
	for _, commit := range commits {
		changed, err := changedFiles(w.repo, commit, w.targetRel())
		if err != nil {
			return nil, err
		}
		if len(productFiles(changed)) > 0 {
			work = append(work, commit)
		}
	}
	return work, nil
}

func (w *world) workCommits(n int) error {
	work, err := w.newWorkCommits()
	if err != nil {
		return err
	}
	if len(work) != n {
		log, _ := git(w.repo, "log", "--stat", "--oneline", w.headBefore+"..HEAD")
		return fmt.Errorf("expected %d new work commits, got %d:\n%s\n%s", n, len(work), log, w.output)
	}
	return nil
}

// eachWorkCommitHoldsOneTask checks every new work commit holds the work
// for one task of the plan, and that no two commits hold the same task's.
func (w *world) eachWorkCommitHoldsOneTask() error {
	work, err := w.newWorkCommits()
	if err != nil {
		return err
	}
	var seen []string
	for _, commit := range work {
		changed, err := changedFiles(w.repo, commit, w.targetRel())
		if err != nil {
			return err
		}
		product := productFiles(changed)
		if len(product) != 1 {
			return fmt.Errorf("commit %s has the product files %v, not the work for one task", commit, product)
		}
		if slices.Contains(seen, product[0]) {
			return fmt.Errorf("%s is the work of more than one commit", product[0])
		}
		seen = append(seen, product[0])
	}
	return nil
}

func committedPlanMatches(target string) error {
	committed, err := git(target, "show", "HEAD:./.factory/plan.md")
	if err != nil {
		return err
	}
	onDisk, err := os.ReadFile(filepath.Join(target, ".factory", "plan.md"))
	if err != nil {
		return err
	}
	if committed != string(trimNewlines(onDisk)) {
		return fmt.Errorf("the committed plan:\n%s\ndiffers from the plan on disk:\n%s", committed, onDisk)
	}
	return nil
}

func trimNewlines(raw []byte) []byte {
	for len(raw) > 0 && (raw[len(raw)-1] == '\n' || raw[len(raw)-1] == '\r') {
		raw = raw[:len(raw)-1]
	}
	return raw
}

func noUncommittedChanges(target string) error {
	status, err := git(target, "status", "--porcelain", "-uall", "--", ".")
	if err != nil {
		return err
	}
	if status != "" {
		return fmt.Errorf("the target %s has uncommitted changes:\n%s", target, status)
	}
	return nil
}

func workCommitted(target string, tasks ...string) error {
	for _, task := range tasks {
		if _, err := git(target, "cat-file", "-e", "HEAD:./"+task+".txt"); err != nil {
			return fmt.Errorf("the work for %s has not been committed: %v", task, err)
		}
	}
	return nil
}

func (w *world) registerTargetSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the target uses the containing repository$`, func() error {
		top, err := git(w.targetDir(), "rev-parse", "--show-toplevel")
		if err != nil {
			return err
		}
		if top != w.repo {
			return fmt.Errorf("the target's repository is %s, not %s", top, w.repo)
		}
		return nil
	})
	sc.Step(`^the target is a Git repository$`, func() error {
		top, err := git(w.targetDir(), "rev-parse", "--show-toplevel")
		if err != nil {
			return err
		}
		if top != w.targetDir() {
			return fmt.Errorf("the target's repository is %s, not the target %s", top, w.targetDir())
		}
		return nil
	})

	sc.Step(`^the factory's own files and unrelated uncommitted changes are as they were$`, func() error {
		var targets []string
		for path := range w.targets {
			targets = append(targets, filepath.Clean(path))
		}
		now, err := stateOutside(w.repo, targets)
		if err != nil {
			return err
		}
		was := w.unrelated
		if changes := was.files.diff(now.files); changes != "" {
			return fmt.Errorf("files outside the targets changed:\n%s", changes)
		}
		if now.status != was.status {
			return fmt.Errorf("git status outside the targets was:\n%s\nand is now:\n%s", was.status, now.status)
		}
		if now.staged != was.staged {
			return fmt.Errorf("what is staged outside the targets was:\n%s\nand is now:\n%s", was.staged, now.staged)
		}
		return nil
	})

	sc.Step(`^the target "([^"]*)" is unchanged$`, func(target string) error {
		before, ok := w.before[target]
		if !ok {
			return fmt.Errorf("the target %q was not recorded before the run", target)
		}
		now, err := snapshot(w.fromRepo(target))
		if err != nil {
			return err
		}
		if changes := before.diff(now); changes != "" {
			return fmt.Errorf("the target %q changed:\n%s", target, changes)
		}
		return nil
	})

	sc.Step(`^the targets "([^"]*)" and "([^"]*)" each have their own completed plan and committed work$`,
		func(first, second string) error {
			for _, target := range []string{first, second} {
				dir := w.fromRepo(target)
				tasks, err := readPlan(filepath.Join(dir, ".factory", "plan.md"))
				if err != nil {
					return err
				}
				if got := names(tasks); !slices.Equal(got, []string{"alpha", "beta"}) {
					return fmt.Errorf("the target %q's plan has the tasks %v", target, got)
				}
				for _, t := range tasks {
					if !t.done {
						return fmt.Errorf("the target %q's plan has %q not done", target, t.name)
					}
				}
				if err := committedPlanMatches(dir); err != nil {
					return err
				}
				if err := workCommitted(dir, "alpha", "beta"); err != nil {
					return fmt.Errorf("in the target %q: %w", target, err)
				}
				if err := noUncommittedChanges(dir); err != nil {
					return err
				}
			}
			return nil
		})
}

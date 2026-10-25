package steps

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/cucumber/godog"
)

func (w *world) registerCommitSteps(sc *godog.ScenarioContext) {
	sc.Step(`^there are two new work commits$`, func() error { return w.workCommits(2) })
	sc.Step(`^there are three new work commits$`, func() error { return w.workCommits(3) })
	sc.Step(`^there are no new work commits$`, func() error { return w.workCommits(0) })
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

	sc.Step(`^the run's folder has been committed in commits of its own$`, w.runFolderCommittedApart)
	sc.Step(`^the run's folder has no uncommitted changes$`, func() error {
		return noUncommittedChanges(w.repo, w.runDir(w.current()))
	})
	sc.Step(`^the work for alpha and beta has been committed$`, func() error {
		return workCommitted(w.targetDir(), "alpha", "beta")
	})
}

// targetRel is the current run's target, as a path from the repository.
func (w *world) targetRel() string {
	rel, _ := filepath.Rel(w.repo, w.fromRepo(w.current().target))
	return rel
}

// targetPaths are the targets the example made, as paths from the repository.
func (w *world) targetPaths() []string {
	var paths []string
	for path := range w.targets {
		paths = append(paths, filepath.Clean(path))
	}
	return paths
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

// noUncommittedChanges checks nothing under the path, new files included, is
// uncommitted.
func noUncommittedChanges(repo, path string) error {
	status, err := git(repo, "status", "--porcelain", "-uall", "--", path)
	if err != nil {
		return err
	}
	if status != "" {
		return fmt.Errorf("%s has uncommitted changes:\n%s", path, status)
	}
	return nil
}

// runFolderCommittedApart checks the latest invocation committed the run's
// folder, and that those commits hold nothing else: the work is in commits
// of its own.
func (w *world) runFolderCommittedApart() error {
	folder, err := filepath.Rel(w.repo, w.runDir(w.current()))
	if err != nil {
		return err
	}
	commits, err := newCommits(w.repo, w.headBefore, folder)
	if err != nil {
		return err
	}
	if len(commits) == 0 {
		return fmt.Errorf("the run's folder %s was not committed\n%s", folder, w.output)
	}
	for _, commit := range commits {
		changed, err := changedFiles(w.repo, commit, ".")
		if err != nil {
			return err
		}
		for _, file := range changed {
			if !underAny(file, []string{folder}) {
				return fmt.Errorf("commit %s holds %s with the run's folder", commit, file)
			}
		}
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
		now, err := stateOutside(w.repo, w.targetPaths())
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

}

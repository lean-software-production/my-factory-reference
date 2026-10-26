package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// excludeLines is the pathspec that leaves a target's .assembly-lines/ out:
// the lines, their machines and their runs are not the target's work.
const excludeLines = ":(exclude)" + linesFolder

// prepareTarget makes a Git repository of the target if no repository
// contains it.
func prepareTarget(target string) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	if _, err := git(target, "rev-parse", "--show-toplevel"); err == nil {
		return nil
	}
	_, err := git(target, "init", "-q")
	return err
}

// commitWork commits the target's work, and nothing else: both adding and
// committing are limited to the target, leaving out its assembly lines, so
// unrelated staged and unstaged changes stay as they are.
func commitWork(target, message string) error {
	return commitPaths(target, message, ".", excludeLines)
}

// commitRunFolder commits the run's folder, and nothing else. It is never
// committed with the work.
func commitRunFolder(f runFolder, message string) error {
	return commitPaths(f.target, message, f.dir)
}

// lockWait is how long a commit waits for a Git that holds the repository's
// index.lock before it gives up.
var lockWait = 10 * time.Second

func commitPaths(dir, message string, paths ...string) error {
	status, err := git(dir, append([]string{"status", "--porcelain", "-uall", "--"}, paths...)...)
	if err != nil || status == "" {
		return err
	}
	if _, err := gitWriting(dir, append([]string{"add", "-A", "--"}, paths...)...); err != nil {
		return err
	}
	_, err = gitWriting(dir, append([]string{"commit", "-q", "-m", message, "--"}, paths...)...)
	return err
}

// gitWriting runs a Git command that takes the repository's index.lock. A
// Git killed while it held the lock, as when the machine it ran on went to
// sleep, never gave it up, and no commit would succeed after it. So when
// the lock is in the way, the command waits for it to go, or removes it if
// no Git is running to hold it, and is run again.
func gitWriting(dir string, args ...string) (string, error) {
	out, err := git(dir, args...)
	if err == nil || !strings.Contains(err.Error(), "index.lock") {
		return out, err
	}
	if lockErr := clearIndexLock(dir); lockErr != nil {
		return "", fmt.Errorf("%w\n%v", err, lockErr)
	}
	return git(dir, args...)
}

// clearIndexLock waits until the repository's index.lock is gone, or until
// no Git is running and it can only be a stale one, which it removes. It
// fails if a Git still runs after lockWait, and leaves the lock then.
func clearIndexLock(dir string) error {
	path, err := git(dir, "rev-parse", "--git-path", "index.lock")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	for deadline := time.Now().Add(lockWait); ; time.Sleep(50 * time.Millisecond) {
		if _, err := os.Stat(path); err != nil {
			return nil // let go in the meantime
		}
		if !gitIsRunning() {
			// A Git that has just finished may have taken it already.
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s is held by a running git: the factory waited %s for it", path, lockWait)
		}
	}
}

// gitIsRunning says whether any Git is running. Where that can't be told it
// says yes, so that no lock is taken for stale.
var gitIsRunning = func() bool {
	err := exec.Command("pgrep", "-x", "git").Run()
	var exit *exec.ExitError
	return err == nil || !errors.As(err, &exit)
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

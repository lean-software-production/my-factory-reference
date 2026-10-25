package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
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

func commitPaths(dir, message string, paths ...string) error {
	status, err := git(dir, append([]string{"status", "--porcelain", "-uall", "--"}, paths...)...)
	if err != nil || status == "" {
		return err
	}
	if _, err := git(dir, append([]string{"add", "-A", "--"}, paths...)...); err != nil {
		return err
	}
	_, err = git(dir, append([]string{"commit", "-q", "-m", message, "--"}, paths...)...)
	return err
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

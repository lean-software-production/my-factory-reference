package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// prepareTarget makes the target folder if it is missing, and a Git
// repository there if no repository contains it.
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

// commitTarget commits the target's work and plan, and nothing else: both
// adding and committing are limited to the target, so unrelated staged and
// unstaged changes stay as they are.
func commitTarget(target, message string) error {
	status, err := git(target, "status", "--porcelain", "-uall", "--", ".")
	if err != nil || status == "" {
		return err
	}
	if _, err := git(target, "add", "-A", "--", "."); err != nil {
		return err
	}
	_, err = git(target, "commit", "-q", "-m", message, "--", ".")
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

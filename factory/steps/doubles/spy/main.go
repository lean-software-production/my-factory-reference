// Command spy stands in for a harness that runs pi. It is called the way the
// factory calls a harness (`spy -p <prompt>`), in the target folder. It
// notes the work in the target and the plan, runs the real pi with the same
// arguments, and notes them again, so a step can tell whether the machine
// changed either. Anything else the machine keeps in the run's folder, in
// the target's .assembly-lines, is neither.
//
// It appends one line to spy.log in the folder named by DOUBLE_DIR: the
// target's state before, then after.
package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	plan := planIn(os.Args[len(os.Args)-1])
	before := state(plan)
	pi := exec.Command("pi", os.Args[1:]...)
	pi.Stdin, pi.Stdout, pi.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := pi.Run()
	after := state(plan)

	log, logErr := os.OpenFile(os.Getenv("DOUBLE_DIR")+"/spy.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if logErr != nil {
		fmt.Fprintln(os.Stderr, "spy:", logErr)
		os.Exit(1)
	}
	fmt.Fprintf(log, "%s %s\n", before, after)
	log.Close()

	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "spy:", err)
		os.Exit(1)
	}
}

// planIn is the plan's path, as the factory's prompt says it.
func planIn(prompt string) string {
	for _, line := range strings.Split(prompt, "\n") {
		for _, prefix := range []string{"The plan is ", "There is no plan yet. It goes in "} {
			if path, ok := strings.CutPrefix(line, prefix); ok {
				return strings.TrimSpace(path)
			}
		}
	}
	return ""
}

// state hashes the plan, and everything about the target's work that Git can
// see: which files changed, how, and what the untracked ones hold.
func state(plan string) string {
	sum := sha256.New()
	if raw, err := os.ReadFile(plan); err == nil {
		sum.Write(raw)
	}
	const work = ":(exclude).assembly-lines"
	for _, args := range [][]string{
		{"status", "--short", "-uall", "--", ".", work},
		{"diff", "HEAD", "--", ".", work},
	} {
		out, _ := exec.Command("git", args...).CombinedOutput()
		sum.Write(out)
	}
	untracked, _ := exec.Command("git", "ls-files", "--others", "--exclude-standard", "-z", "--", ".", work).Output()
	for _, name := range splitNul(untracked) {
		raw, _ := os.ReadFile(name)
		fmt.Fprintf(sum, "%s\n", name)
		sum.Write(raw)
	}
	return fmt.Sprintf("%x", sum.Sum(nil))
}

func splitNul(raw []byte) []string {
	var names []string
	start := 0
	for i, b := range raw {
		if b == 0 {
			names = append(names, string(raw[start:i]))
			start = i + 1
		}
	}
	return names
}

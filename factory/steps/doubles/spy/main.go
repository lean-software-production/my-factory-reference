// Command spy stands in for a harness that runs pi. It is called the way the
// factory calls a harness (`spy -p <prompt>`), in the target folder. It
// notes what the target holds, runs the real pi with the same arguments,
// and notes it again, so a step can tell whether the machine changed
// anything.
//
// It appends one line to spy.log in the folder named by DOUBLE_DIR: the
// target's state before, then after.
package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	before := state()
	pi := exec.Command("pi", os.Args[1:]...)
	pi.Stdin, pi.Stdout, pi.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := pi.Run()
	after := state()

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

// state hashes everything about the folder that Git can see: which files
// changed, how, and what the untracked ones hold.
func state() string {
	sum := sha256.New()
	for _, args := range [][]string{
		{"status", "--short", "-uall", "--", "."},
		{"diff", "HEAD", "--", "."},
	} {
		out, _ := exec.Command("git", args...).CombinedOutput()
		sum.Write(out)
	}
	untracked, _ := exec.Command("git", "ls-files", "--others", "--exclude-standard", "-z", "--", ".").Output()
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

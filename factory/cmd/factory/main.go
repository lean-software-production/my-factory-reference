// Command factory turns a seed into working software by running an assembly
// line: a graph of machines it reads before it does any work. On the line as
// it stands, a planner writes the plan and keeps it, a doer does each task
// and a validator checks the doer's work.
//
// The factory goes along the line from start, calling each machine it comes
// to and taking the edge its result names, until the line reaches finish.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// version is set by the release build with -ldflags "-X main.version=<v>".
var version = "dev"

func main() {
	flags := flag.NewFlagSet("factory", flag.ContinueOnError)
	seed := flags.String("seed", "", "the seed file that says what to build")
	target := flags.String("target", "", "the folder to build it in")
	attempts := flags.Int("attempts", 3, "the most times the line may send a task to the same machine before giving up")
	if err := flags.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	if err := runFactory(*seed, *target, *attempts); err != nil {
		fmt.Fprintln(os.Stderr, "factory:", err)
		os.Exit(1)
	}
}

func runFactory(seed, target string, attempts int) error {
	if target == "" {
		return errors.New("a target is required: give one with --target <folder>")
	}
	if seed == "" {
		return errors.New("there is no seed: give one with --seed <file>")
	}
	// Relative paths are the caller's: resolve them from the working directory.
	seed, err := filepath.Abs(seed)
	if err != nil {
		return err
	}
	if info, err := os.Stat(seed); err != nil || info.IsDir() {
		return fmt.Errorf("there is no seed at %s", seed)
	}
	if target, err = filepath.Abs(target); err != nil {
		return err
	}
	// The line is read, and refused if it cannot be run, before any work.
	line, err := readAssemblyLine()
	if err != nil {
		return err
	}
	machines, err := loadMachines(line)
	if err != nil {
		return err
	}
	// A machine that cannot sign in is reported before any machine is called.
	for _, name := range line.machinesOn() {
		if err := machines[name].checkSignIn(); err != nil {
			return err
		}
	}
	if err := prepareTarget(target); err != nil {
		return err
	}
	r := &run{line, machines, attempts, seed, target, filepath.Join(target, ".factory", "plan.md")}
	return r.along()
}

// Command factory turns a seed into working software by driving a coding agent.
//
// Each pass calls the agent in the target folder, pointing it at the seed and
// the target's plan. The agent writes the plan, or does the next task and
// marks it done, and answers with a result. The factory commits the target's
// work and stops when the result says the plan is complete.
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
	agent := flags.String("agent", "pi", "the coding agent to run; it is called as: <agent> -p <prompt>")
	all := flags.Bool("all", false, "keep going until the plan is complete")
	if err := flags.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	if err := run(*seed, *target, *agent, *all); err != nil {
		fmt.Fprintln(os.Stderr, "factory:", err)
		os.Exit(1)
	}
}

func run(seed, target, agent string, all bool) error {
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
	if err := prepareTarget(target); err != nil {
		return err
	}

	for {
		result, err := pass(agent, seed, target)
		if err != nil {
			return err
		}
		if result.complete() {
			fmt.Println("factory stopped")
			return nil
		}
		if !all {
			return nil
		}
	}
}

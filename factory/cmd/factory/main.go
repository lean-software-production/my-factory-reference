// Command factory turns a seed into working software by driving three
// machines: a planner that keeps the plan, a doer that does each task and a
// validator that checks the doer's work.
//
// Each pass has the doer do the next task, in the target folder, until the
// validator is satisfied, then commits the work. The planner then marks the
// task done, and its result says when the plan is complete.
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
	agent := flags.String("agent", "pi", "the harness that runs every machine; it is called as: <harness> -p <prompt>")
	planner := flags.String("planner", "", "the harness that runs the planner, if not --agent")
	doer := flags.String("doer", "", "the harness that runs the doer, if not --agent")
	validator := flags.String("validator", "", "the harness that runs the validator, if not --agent")
	attempts := flags.Int("attempts", 3, "the most times a pass lets the doer try a task before giving up")
	lens := flags.String("lens", "single responsibility", "what the validator looks for in the work")
	all := flags.Bool("all", false, "keep going until the plan is complete")
	if err := flags.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	pick := func(role, harness string) machine {
		if harness == "" {
			harness = *agent
		}
		return machine{role, harness}
	}
	set := settings{
		planner:   pick("planner", *planner),
		doer:      pick("doer", *doer),
		validator: pick("validator", *validator),
		attempts:  *attempts,
		lens:      *lens,
	}
	if err := run(*seed, *target, set, *all); err != nil {
		fmt.Fprintln(os.Stderr, "factory:", err)
		os.Exit(1)
	}
}

func run(seed, target string, set settings, all bool) error {
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

	for checked := false; ; checked = true {
		result, err := pass(set, seed, target, checked)
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

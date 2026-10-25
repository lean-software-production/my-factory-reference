// Command factory turns a seed into working software by running an assembly
// line: a graph of machines it reads before it does any work. On the line as
// it stands, a planner writes the plan and keeps it, a doer does each task
// and a validator checks the doer's work.
//
// A run is a named execution of one of a target's assembly lines on a seed.
// The lines, their machines and the runs all live in the target, in
// .assembly-lines/; the factory keeps nothing of them itself.
//
// The factory goes along the line from start, calling each machine it comes
// to and taking the edge its result names, until the line reaches finish.
// Where the line fans out, the branches run at the same time, and the
// machine where they meet is given their results.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// version is set by the release build with -ldflags "-X main.version=<v>".
var version = "dev"

func main() {
	flags := flag.NewFlagSet("factory", flag.ContinueOnError)
	var invocation invocation
	flags.StringVar(&invocation.run, "run", "", "the name of the run")
	flags.StringVar(&invocation.target, "target", "", "the folder that holds the run's lines and that it builds in")
	flags.StringVar(&invocation.line, "line", "", "the target's assembly line the run executes (the first time only)")
	flags.StringVar(&invocation.seed, "seed", "", "the seed file that says what to build (the first time only)")
	flags.IntVar(&invocation.attempts, "attempts", 3, "the most times the line may send a task to the same machine before giving up")
	if err := flags.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	// Interrupted, or its terminal closed, the factory stops the machines
	// running before it stops itself.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	if err := invocation.execute(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "factory:", err)
		stop()
		os.Exit(1)
	}
}

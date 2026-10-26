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
// machine where they meet is given their results. Each machine is an ACP
// agent.
//
// The factory runs as a daemon, one run at a time. Starting a run returns
// at once; watching it shows what the machines generate and spend, from the
// run's record; a machine can be told something while it works; and runs,
// and the factory, can be stopped:
//
//	factory --run <name> --target <folder> [--line <line> --seed <file>] [--attempts <n>]
//	factory watch <name> --target <folder>
//	factory say <name> --target <folder> <machine> <message>
//	factory stop <name> --target <folder>
//	factory stop
//	factory status
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// version is set by the release build with -ldflags "-X main.version=<v>".
var version = "dev"

func main() {
	os.Exit(command(os.Args[1:]))
}

// command runs the command the arguments name, and is its exit status.
func command(args []string) int {
	commands := map[string]func([]string) error{
		"watch":  watchCommand,
		"say":    sayCommand,
		"stop":   stopCommand,
		"status": statusCommand,
		"daemon": daemonCommand,
	}
	run := startCommand
	if len(args) > 0 {
		if c, ok := commands[args[0]]; ok {
			run, args = c, args[1:]
		}
	}
	err := run(args)
	var status exitStatus
	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return 0
	case errors.As(err, &status):
		return int(status)
	}
	fmt.Fprintln(os.Stderr, "factory:", err)
	return 1
}

// exitStatus is a command's failure that it has already explained.
type exitStatus int

func (s exitStatus) Error() string { return fmt.Sprintf("exit status %d", int(s)) }

// startCommand starts or resumes a run, and returns as soon as the factory
// has started it.
func startCommand(args []string) error {
	flags := flag.NewFlagSet("factory", flag.ContinueOnError)
	var i invocation
	flags.StringVar(&i.run, "run", "", "the name of the run")
	flags.StringVar(&i.target, "target", "", "the folder that holds the run's lines and that it builds in")
	flags.StringVar(&i.line, "line", "", "the target's assembly line the run executes (the first time only)")
	flags.StringVar(&i.seed, "seed", "", "the seed file that says what to build (the first time only)")
	flags.IntVar(&i.attempts, "attempts", 3, "the most times the line may send a task to the same machine before giving up")
	if err := flags.Parse(args); err != nil {
		return usage(err)
	}
	if flags.NArg() > 0 {
		return usage(fmt.Errorf("unexpected %q", flags.Arg(0)))
	}
	i, err := i.resolved()
	if err != nil {
		return err
	}
	rep, err := call(request{Op: "start", Run: i.run, Target: i.target, Line: i.line, Seed: i.seed,
		Attempts: i.attempts, Env: os.Environ()}, true)
	if err != nil {
		return err
	}
	fmt.Println(rep.Message)
	return nil
}

// usage is a command line the factory cannot read. The flag package has
// said why, if it was the one that could not.
func usage(err error) error {
	if errors.Is(err, flag.ErrHelp) {
		return err
	}
	if !strings.HasPrefix(err.Error(), "flag") {
		fmt.Fprintln(os.Stderr, "factory:", err)
	}
	return exitStatus(2)
}

// runFlags reads the arguments of a command about a run: its name, before
// or after the flags, and its target.
func runFlags(name string, args []string) (invocation, error) {
	i, rest, err := runArgs(name, args)
	if err == nil && len(rest) > 0 {
		err = usage(fmt.Errorf("unexpected %q", rest[0]))
	}
	return i, err
}

// runArgs reads the arguments of a command about a run, with the flags
// anywhere among them: the run's name, the first of the rest, and its
// target. It returns the arguments after the name. After "--", none is a
// flag.
func runArgs(name string, args []string) (invocation, []string, error) {
	flags := flag.NewFlagSet("factory "+name, flag.ContinueOnError)
	var i invocation
	flags.StringVar(&i.target, "target", "", "the folder that holds the run")
	var rest []string
	for len(args) > 0 {
		if !strings.HasPrefix(args[0], "-") || args[0] == "-" {
			rest, args = append(rest, args[0]), args[1:]
			continue
		}
		if args[0] == "--" {
			rest = append(rest, args[1:]...)
			break
		}
		if err := flags.Parse(args); err != nil {
			return i, nil, usage(err)
		}
		if parsed := args[:len(args)-flags.NArg()]; parsed[len(parsed)-1] == "--" {
			rest = append(rest, flags.Args()...)
			break
		}
		args = flags.Args()
	}
	if len(rest) > 0 {
		i.run, rest = rest[0], rest[1:]
	}
	i, err := i.resolved()
	return i, rest, err
}

// sayCommand passes a message to a machine of the run while it works, and
// returns once the machine has it. It is refused if that machine is not
// working now.
func sayCommand(args []string) error {
	i, rest, err := runArgs("say", args)
	if err != nil {
		return err
	}
	if len(rest) != 2 {
		return usage(errors.New("say names the run, the machine and the message: factory say <run> --target <folder> <machine> <message>"))
	}
	machine, message := rest[0], rest[1]
	rep, err := call(request{Op: "say", Run: i.run, Target: i.target, Machine: machine, Message: message}, false)
	if errors.Is(err, errNotRunning) {
		return errors.New("refused: nothing is running: the factory is not running")
	}
	if err != nil {
		return err
	}
	fmt.Println(rep.Message)
	return nil
}

// stopCommand stops the run it names, leaving the factory running, or, if it
// names none, the factory and its run. It returns once they have stopped.
func stopCommand(args []string) error {
	if len(args) == 0 {
		rep, err := call(request{Op: "shutdown"}, false)
		if errors.Is(err, errNotRunning) {
			fmt.Println("the factory is not running")
			return nil
		}
		if err != nil {
			return err
		}
		if !waitUntilGone(rep.PID, daemonStartLimit) {
			return fmt.Errorf("the factory (process %d) said it had stopped, and is still running", rep.PID)
		}
		fmt.Println(rep.Message)
		return nil
	}
	i, err := runFlags("stop", args)
	if err != nil {
		return err
	}
	rep, err := call(request{Op: "stop", Run: i.run, Target: i.target}, false)
	if errors.Is(err, errNotRunning) {
		return fmt.Errorf("the %q run is not running: the factory is not running", i.run)
	}
	if err != nil {
		return err
	}
	fmt.Println(rep.Message)
	return nil
}

// statusCommand says whether the factory is running, and which run.
func statusCommand(args []string) error {
	if len(args) > 0 {
		return usage(fmt.Errorf("unexpected %q", args[0]))
	}
	rep, err := call(request{Op: "status"}, false)
	if errors.Is(err, errNotRunning) {
		fmt.Println("the factory is not running")
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Printf("the factory is running (process %d)\n", rep.PID)
	if rep.Run == "" {
		fmt.Println("no run is running")
	} else {
		fmt.Printf("run %s is running, in %s\n", rep.Run, rep.Target)
	}
	return nil
}

// daemonCommand is the factory itself, which the other commands start when
// they need it.
func daemonCommand(args []string) error {
	flags := flag.NewFlagSet("factory daemon", flag.ContinueOnError)
	readyFD := flags.Int("ready-fd", 0, "a file descriptor to say on when the factory is ready")
	if err := flags.Parse(args); err != nil {
		return usage(err)
	}
	var ready *os.File
	if *readyFD > 2 {
		ready = os.NewFile(uintptr(*readyFD), "ready")
	}
	return serveDaemon(ready)
}

// stoppedBySignals is a context that is done when the factory is
// interrupted, terminated or hung up on, so that it stops its run before
// it stops itself. A signal the factory was started with ignored stays
// ignored: started with nohup, or in the background from a script, it
// carries on as it always did.
func stoppedBySignals() (context.Context, context.CancelFunc) {
	var watched []os.Signal
	for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP} {
		if !signal.Ignored(sig) {
			watched = append(watched, sig)
		}
	}
	if len(watched) == 0 {
		// Notified of no signal in particular, a context would be done on any.
		return context.WithCancel(context.Background())
	}
	return signal.NotifyContext(context.Background(), watched...)
}

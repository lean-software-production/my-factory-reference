package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/lean-software-production/my-factory-reference/factory/internal/machine"
	"github.com/lean-software-production/my-factory-reference/factory/internal/watch"
)

// result is what a machine says it did: the last line of its answer that
// is JSON.
type result map[string]any

// prepare reads and checks what the run the invocation names needs, before
// any machine is called: the line is read, and refused if it cannot be run,
// and each machine is checked, before any work is done or anything is
// written. Then the target is made a repository if it is in none, and a run
// that has not been started is started: its folder is saved and committed.
// Its machines run in the environment env, the caller's.
func (i invocation) prepare(env []string) (*run, error) {
	folder, err := i.openRun()
	if err != nil {
		return nil, err
	}
	line, err := loadAssemblyLine(folder.lineDir())
	if err != nil {
		return nil, err
	}
	machines, err := loadMachines(line)
	if err != nil {
		return nil, err
	}
	// A machine that cannot be run, or cannot sign in, is reported before
	// any machine is called.
	for _, name := range line.machinesOn() {
		if err := machines[name].CheckHarness(env); err != nil {
			return nil, err
		}
		if err := machines[name].CheckSignIn(env); err != nil {
			return nil, err
		}
	}
	if err := prepareTarget(folder.target); err != nil {
		return nil, err
	}
	if !folder.started {
		if err := folder.save(); err != nil {
			return nil, err
		}
		if err := commitRunFolder(folder, "Start the "+folder.name+" run"); err != nil {
			return nil, err
		}
	}
	return &run{line: line, machines: machines, attempts: i.attempts, folder: folder, env: env}, nil
}

// A run is the factory going along the assembly line from start, calling
// each machine it comes to and taking the edge its result names, until the
// line reaches finish.
type run struct {
	line     *assemblyLine
	machines map[string]machine.Machine
	attempts int // the most times a task may send the line back to a machine
	folder   runFolder
	env      []string // the environment its machines run in
	record   *record  // what its machines are given, generate and spend
}

// A task is finished when the line comes back to the planner with the work
// done: the work is committed then, and the planner records it in the plan.
func (r *run) along(ctx context.Context) error {
	const defaultMessage = "Do the next task"
	var (
		planner = r.line.planner()
		target  = r.folder.target
		node    = start
		res     = result{}
		before  []answer // what the machines before the next one answered
		met     bool     // whether the next machine is where branches meet
		message = defaultMessage
		entered = map[string]int{} // how many times each machine was called for this task
	)
	// enter counts a call to the machine for this task, or says the task has
	// hit its limit. The line coming back to the planner commits the task's
	// work and starts the next.
	enter := func(machine string) error {
		if machine == planner {
			if node != start {
				if err := commitWork(target, message); err != nil {
					return err
				}
			}
			entered, message = map[string]int{}, defaultMessage
			return nil
		}
		if entered[machine]++; entered[machine] > r.attempts {
			return fmt.Errorf("a task hit its limit of %d attempts: the assembly line sent it to the %s again without getting it done", r.attempts, machine)
		}
		return nil
	}

	for {
		var next string
		if branches := r.line.branches(node); branches != nil {
			for _, branch := range branches {
				if err := enter(branch); err != nil {
					return err
				}
			}
			shared, err := r.briefing(node, before)
			if err != nil {
				return err
			}
			if before, err = r.fanOut(ctx, node, branches, shared); err != nil {
				return err
			}
			next, _ = r.line.meeting(branches)
			met = true
		} else {
			var err error
			if next, err = r.line.next(node, res); err != nil {
				return err
			}
			met = false
		}
		if next == finish {
			return nil
		}
		if err := enter(next); err != nil {
			return err
		}

		planned := fileExists(r.folder.plan())
		var prompt string
		if met {
			prompt = meetingBriefing{before, r.fields(next)}.String()
		} else {
			b, err := r.briefing(node, before)
			if err != nil {
				return err
			}
			b.fields = r.fields(next)
			prompt = b.String()
		}
		said, got, err := r.ask(ctx, next, prompt)
		if err != nil {
			return err
		}
		res, before = got, []answer{{next, said}}
		if next == planner {
			commit := "Update the plan"
			if !planned {
				commit = "Write the plan"
			}
			if err := commitRunFolder(r.folder, commit); err != nil {
				return err
			}
		} else if task, ok := res["task"].(string); ok && task != "" && message == defaultMessage && r.line.isWorker(next) {
			message = task
		}
		node = next
	}
}

// briefing is what every machine is told, whatever it is for, about where
// the run stands now, after the machine on the line before it. It is asked
// for no fields yet.
func (r *run) briefing(node string, before []answer) (briefing, error) {
	changes, err := git(r.folder.target, "status", "--short", "-uall", "--", ".", excludeLines)
	if err != nil {
		return briefing{}, err
	}
	b := briefing{seed: r.folder.seedPath(), plan: r.folder.plan(), runDir: r.folder.dir,
		planExists: fileExists(r.folder.plan()), changes: changes}
	if node != start && len(before) == 1 {
		b.before, b.answer = before[0].machine, before[0].result
	}
	return b, nil
}

// fields are the fields of its result a machine is asked for.
func (r *run) fields(machine string) []field {
	return resultFields(r.line.routedFields(machine), r.line.isWorker(machine), r.line.canSendWorkBack(machine))
}

// fanOut runs the branches from a node at the same time and waits for all
// of them. Each is told the same about where the run stands and asked for
// its own result. The first branch that fails stops the others, and the
// fan out fails with its error.
func (r *run) fanOut(ctx context.Context, from string, branches []string, shared briefing) ([]answer, error) {
	parent := ctx
	ctx, stop := context.WithCancelCause(ctx)
	defer stop(nil)
	answers := make([]answer, len(branches))
	var wg sync.WaitGroup
	for i, branch := range branches {
		b := shared
		b.fields = r.fields(branch)
		prompt := b.String()
		wg.Go(func() {
			said, _, err := r.ask(ctx, branch, prompt)
			if err != nil {
				stop(err)
				return
			}
			answers[i] = answer{branch, said}
		})
	}
	wg.Wait()
	if parent.Err() != nil {
		return nil, fmt.Errorf("the factory was stopped while the branches from %s ran: %w", from, context.Cause(parent))
	}
	if err := context.Cause(ctx); err != nil {
		return nil, fmt.Errorf("a branch from %s failed, so the factory stopped the others: %w", from, err)
	}
	return answers, nil
}

// answer is a machine's result, as it said it.
type answer struct {
	machine, result string
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ask runs a machine and reads its result. Everything it is given,
// generates and spends goes in the run's record as it happens.
func (r *run) ask(ctx context.Context, name, prompt string) (string, result, error) {
	m := r.machines[name]
	r.record.add(watch.Event{Kind: watch.Start, Machine: name, Prompt: prompt})
	turn, err := m.Run(ctx, r.folder.target, prompt, r.env, func(line string) {
		r.record.add(watch.Event{Kind: watch.Text, Machine: name, Text: line})
	})
	if err == nil {
		r.record.add(watch.Event{Kind: watch.Usage, Machine: name, Provider: m.Provider(), Model: m.Model,
			InputTokens: turn.Usage.InputTokens, OutputTokens: turn.Usage.OutputTokens})
	}
	line, res := lastResult(turn.Text)
	if err == nil && res == nil {
		err = fmt.Errorf("could not read the %s's result: no line of its answer is a JSON object:\n%s", m.Name, turn.Text)
	}
	if err != nil {
		r.record.add(watch.Event{Kind: watch.End, Machine: name, Error: err.Error()})
		return "", nil, err
	}
	r.record.add(watch.Event{Kind: watch.Result, Machine: name, Text: line, Result: res})
	r.record.add(watch.Event{Kind: watch.End, Machine: name})
	return line, res, nil
}

// lastResult finds the last line of the answer that is a JSON object.
func lastResult(answer string) (string, result) {
	lines := strings.Split(answer, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		var res result
		if json.Unmarshal([]byte(line), &res) == nil && res != nil {
			return line, res
		}
	}
	return "", nil
}

// loadMachines reads the folders of the machines on the line.
func loadMachines(l *assemblyLine) (map[string]machine.Machine, error) {
	machines := map[string]machine.Machine{}
	for _, name := range l.machinesOn() {
		m, err := machine.Load(name, l.machineDir(name))
		if err != nil {
			return nil, err
		}
		machines[name] = m
	}
	return machines, nil
}

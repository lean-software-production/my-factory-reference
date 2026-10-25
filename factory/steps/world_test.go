package steps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// behaviour is what the agent double does; see doubles/agent.
type behaviour struct {
	Tasks             []string `json:"tasks"`
	PlannerProse      bool     `json:"planner_prose,omitempty"`
	DoerProse         bool     `json:"doer_prose,omitempty"`
	ProductFile       string   `json:"product_file,omitempty"`
	ValidatorSays     string   `json:"validator_says,omitempty"`
	ValidatorNoResult bool     `json:"validator_no_result,omitempty"`
	Rejects           int      `json:"rejects,omitempty"`
	Finding           string   `json:"finding,omitempty"`
}

// A run of the factory that the double plays the agent in is short; one
// that a real agent does is not.
const (
	doubleRunLimit = 30 * time.Second
	realRunLimit   = 55 * time.Minute
)

// world is one example's state. Each example has its own workspace: a
// test repository holding a copy of the factory, and a folder for the
// doubles.
type world struct {
	ws        string // the example's temporary folder, removed when it ends
	repo      string // the test repository, where the factory is run from
	doubleDir string // where the doubles log their calls and keep state

	seed      string // as given on the command line, from repo
	target    string // as given on the command line, from repo
	noSeed    bool
	noTarget  bool
	agent     string // the harness chosen for every machine on the command line, if any
	doer      string // the harness chosen for the doer alone, if any
	validator string // the harness chosen for the validator alone, if any
	attempts  int    // the most attempts a pass is allowed, if set
	lens      string // what the validator looks for, if chosen
	realPi    bool   // the agent is the real pi
	does      behaviour

	planTasks []string          // the tasks of a plan an example wrote
	targets   map[string]string // every target this example named, by path from repo

	// The latest run, and the state just before it.
	output     string
	exitErr    error
	timedOut   bool
	headBefore string
	before     map[string]files // targets' files before the run
	unrelated  *repoState       // the factory's files and unrelated changes, once set up

	// The latest game started in the target.
	screen string
	board  *board
}

func newWorld() *world {
	return &world{targets: map[string]string{}}
}

func (w *world) planPath() string      { return filepath.Join(w.targetDir(), ".factory", "plan.md") }
func (w *world) seedPath() string      { return w.fromRepo(w.seed) }
func (w *world) targetDir() string     { return w.fromRepo(w.target) }
func (w *world) factoryDir() string    { return filepath.Join(w.repo, "factory") }
func (w *world) behaviourFile() string { return filepath.Join(w.doubleDir, "behaviour.json") }

func (w *world) fromRepo(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(w.repo, path)
}

// setUp makes the workspace: a repository with a copy of the factory and
// nothing else, committed.
func (w *world) setUp() error {
	ws, err := os.MkdirTemp("", "factory-example-")
	if err != nil {
		return err
	}
	// Git reports real paths, so the workspace is known by its real path.
	if w.ws, err = filepath.EvalSymlinks(ws); err != nil {
		return err
	}
	w.repo = filepath.Join(ws, "repo")
	w.doubleDir = filepath.Join(ws, "double")
	w.agent = agentDouble
	for _, dir := range []string{w.factoryDir(), filepath.Join(w.repo, "bin"), w.doubleDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := copyFile(factoryBinary, filepath.Join(w.factoryDir(), "factory"), 0o755); err != nil {
		return err
	}
	if err := os.Symlink("../factory/factory", filepath.Join(w.repo, "bin", "factory")); err != nil {
		return err
	}
	if _, err := git(w.repo, "init", "-q"); err != nil {
		return err
	}
	return commitAll(w.repo, "A copy of the factory", ".")
}

func (w *world) tearDown() {
	if w.ws != "" {
		os.RemoveAll(w.ws)
	}
}

// useTarget selects a target by its path from the repository.
func (w *world) useTarget(path string) {
	w.target = path
	if !filepath.IsAbs(path) {
		w.targets[path] = w.fromRepo(path)
	}
}

// run runs the factory once from the repository root, through bin/factory,
// as a student would.
func (w *world) run(all bool) error {
	var args []string
	if !w.noSeed {
		args = append(args, "--seed", w.seed)
	}
	if !w.noTarget {
		args = append(args, "--target", w.target)
	}
	if w.agent != "" && !w.realPi {
		args = append(args, "--agent", w.agent)
	}
	if w.doer != "" {
		args = append(args, "--doer", w.doer)
	}
	if w.validator != "" {
		args = append(args, "--validator", w.validator)
	}
	if w.attempts > 0 {
		args = append(args, "--attempts", strconv.Itoa(w.attempts))
	}
	if w.lens != "" {
		args = append(args, "--lens", w.lens)
	}
	if all {
		args = append(args, "--all")
	}

	raw, err := json.Marshal(w.does)
	if err != nil {
		return err
	}
	if err := os.WriteFile(w.behaviourFile(), raw, 0o644); err != nil {
		return err
	}
	if w.headBefore, err = git(w.repo, "rev-parse", "HEAD"); err != nil {
		return err
	}
	w.before = map[string]files{}
	for path, dir := range w.targets {
		if w.before[path], err = snapshot(dir); err != nil {
			return err
		}
	}

	limit := doubleRunLimit
	env := append(os.Environ(), "DOUBLE_DIR="+w.doubleDir)
	if w.realPi {
		limit = realRunLimit
	} else {
		env = append(env, "PATH="+fakePiDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(w.repo, "bin", "factory"), args...)
	cmd.Dir = w.repo
	cmd.Env = env
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if w.realPi {
		cmd.Stdout = io.MultiWriter(&out, os.Stdout)
		cmd.Stderr = io.MultiWriter(&out, os.Stderr)
	}
	w.exitErr = cmd.Run()
	w.output = out.String()
	w.timedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
	return nil
}

// runToSuccess runs the factory to completion, and fails unless it succeeds.
func (w *world) runToSuccess() error {
	if err := w.run(true); err != nil {
		return err
	}
	if w.exitErr != nil || w.timedOut {
		return fmt.Errorf("the factory did not finish (%v):\n%s", w.exitErr, w.output)
	}
	return nil
}

// call is one call a double was given: the name it was called by, and the
// machine it was asked to be.
type call struct {
	name, role string
	prompt     string
}

// calls lists the calls the doubles were given, in order.
func (w *world) calls() ([]call, error) {
	raw, err := os.ReadFile(filepath.Join(w.doubleDir, "calls.log"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var calls []call
	for n, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("calls.log line %d is %q", n+1, line)
		}
		prompt, err := os.ReadFile(filepath.Join(w.doubleDir, "prompts", fmt.Sprintf("%d.txt", n+1)))
		if err != nil {
			return nil, err
		}
		calls = append(calls, call{fields[0], fields[1], string(prompt)})
	}
	return calls, nil
}

// callsTo lists the calls made to the machine in that role.
func (w *world) callsTo(role string) ([]call, error) {
	calls, err := w.calls()
	var mine []call
	for _, c := range calls {
		if c.role == role {
			mine = append(mine, c)
		}
	}
	return mine, err
}

// lastPrompt is what the machine in that role was last given.
func (w *world) lastPrompt(role string) (string, error) {
	calls, err := w.callsTo(role)
	if err != nil {
		return "", err
	}
	if len(calls) == 0 {
		return "", fmt.Errorf("the %s was never called", role)
	}
	return calls[len(calls)-1].prompt, nil
}

// reports checks that the factory's output says what it should.
func (w *world) reports(words string) error {
	if !strings.Contains(w.output, words) {
		return fmt.Errorf("expected the factory to report %q, but it said:\n%s", words, w.output)
	}
	return nil
}

func copyFile(from, to string, mode os.FileMode) error {
	raw, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, raw, mode)
}

// InitializeScenario registers the step definitions, with a fresh world
// for every example.
func InitializeScenario(sc *godog.ScenarioContext) {
	w := newWorld()
	sc.After(func(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		w.tearDown()
		return ctx, nil
	})
	w.registerSetupSteps(sc)
	w.registerRunSteps(sc)
	w.registerPlanSteps(sc)
	w.registerCommitSteps(sc)
	w.registerTargetSteps(sc)
	w.registerRealAgentSteps(sc)
	w.registerValidationSteps(sc)
}

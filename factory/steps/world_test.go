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
	"regexp"
	"slices"
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

	ReviewersWait      bool              `json:"reviewers_wait,omitempty"`
	NumberedReports    bool              `json:"numbered_reports,omitempty"`
	ReviewerFindings   map[string]string `json:"reviewer_findings,omitempty"`
	Prose              []string          `json:"prose,omitempty"`
	SynthesiserRejects int               `json:"synthesiser_rejects,omitempty"`
	SynthesiserFinding string            `json:"synthesiser_finding,omitempty"`

	DoerSays     []string `json:"doer_says,omitempty"`
	DoerThenSays []string `json:"doer_then_says,omitempty"`
}

// A run of the factory that the double plays the agent in is short; one
// that a real agent does is not.
const (
	doubleRunLimit = 30 * time.Second
	realRunLimit   = 55 * time.Minute
)

// runSpec is a run an example names: the line it executes, and the seed and
// target it is given, as paths from the repository.
type runSpec struct {
	name, line, seed, target string
	tasks                    []string // the tasks of the plan an example wrote for it, if it did
}

// world is one example's state. Each example has its own workspace: a
// test repository holding a copy of the factory, and a folder for the
// doubles. The targets, with their assembly lines, are in the repository.
type world struct {
	ws        string // the example's temporary folder, removed when it ends
	repo      string // the test repository, where the factory is run from
	doubleDir string // where the doubles log their calls and keep state

	seed     string // the latest seed an example wrote, as a path from repo
	target   string // the latest target an example made, as a path from repo
	lineDirs []string
	runs     []*runSpec // the runs an example named, in order; the last is the current one
	ran      *runSpec   // the run the factory was last asked to run
	noTarget bool       // the next invocation is given no target
	attempts int        // the most attempts a task is allowed, if set
	realPi   bool       // the machines run the real pi
	env      []string   // set in the factory's environment, as NAME=value
	subject  string     // the machine the latest "was given" step was about
	does     behaviour

	planTasks []string        // the tasks of the plan an example wrote for the current run
	targets   map[string]bool // every target this example made, by path from repo

	// The latest invocation, and the state just before it.
	output     string
	exitErr    error
	timedOut   bool
	headBefore string
	unrelated  *repoState // the factory's files and unrelated changes, once set up

	// What the latest watch command has shown, while it runs and after.
	watching *watcher
	// The latest run's record, as read.
	record []recordEvent

	// The latest game started in the target.
	screen string
	board  *board
}

func newWorld() *world {
	return &world{targets: map[string]bool{}}
}

func (w *world) factoryDir() string    { return filepath.Join(w.repo, "factory") }
func (w *world) behaviourFile() string { return filepath.Join(w.doubleDir, "behaviour.json") }

func (w *world) fromRepo(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(w.repo, path)
}

// current is the run the latest "a run named" step named.
func (w *world) current() *runSpec { return w.runs[len(w.runs)-1] }

func (w *world) runNamed(name string) (*runSpec, error) {
	for _, r := range w.runs {
		if r.name == name {
			return r, nil
		}
	}
	return nil, fmt.Errorf("no run named %q has been given", name)
}

func (w *world) targetDir() string { return w.fromRepo(w.target) }
func (w *world) seedPath() string  { return w.fromRepo(w.seed) }

// The current run's seed, target and plan; the run's folder is
// .assembly-lines/<line>/runs/<name>/ in its target.
func (w *world) runDir(r *runSpec) string {
	return filepath.Join(w.fromRepo(r.target), ".assembly-lines", r.line, "runs", r.name)
}
func (w *world) planOf(r *runSpec) string { return filepath.Join(w.runDir(r), "plan.md") }
func (w *world) planPath() string         { return w.planOf(w.current()) }

// setUp makes the workspace: a repository with a copy of the factory and
// nothing else, committed. The factory has no assembly lines of its own: they
// are in the targets.
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

// tearDown stops what the example started, the factory included, and
// removes its workspace. It fails the example if a machine was still
// running once the factory had stopped.
func (w *world) tearDown() error {
	var errs []error
	if w.watching != nil {
		w.watching.stop()
	}
	if w.repo != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if out, err := w.factory(ctx, "stop"); err != nil {
			errs = append(errs, fmt.Errorf("stopping the factory: %v\n%s", err, out))
		}
		errs = append(errs, w.noMachineRunning())
	}
	if w.ws != "" {
		os.RemoveAll(w.ws)
	}
	return errors.Join(errs...)
}

// useTarget makes the target, a folder in the repository, the current one.
func (w *world) useTarget(path string) error {
	w.target = path
	w.targets[path] = true
	return os.MkdirAll(w.fromRepo(path), 0o755)
}

// invocation is how the factory is asked to run a run.
type invocation struct {
	line          string // given instead of the run's own, if set
	nameAndTarget bool   // given only the run's name and target
}

// run runs the current run, as the latest "a run named" step named it.
func (w *world) run() error { return w.runRun(w.current(), invocation{}) }

// runRun runs the run to the end, as the factory runs a run from homework
// 7: it starts it, then waits until it has finished, by watching it. What
// the factory said is what both commands said. Nothing is held part-way.
func (w *world) runRun(r *runSpec, how invocation) error {
	if err := w.releaseHolds(); err != nil {
		return err
	}
	limit := doubleRunLimit
	if w.realPi {
		limit = realRunLimit
	}
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	if err := w.start(ctx, r, how); err != nil {
		return err
	}
	if w.exitErr == nil {
		var watched string
		watched, w.exitErr = w.factory(ctx, "watch", r.name, "--target", r.target)
		w.output += watched
	}
	w.timedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
	return nil
}

// start has the factory start the run, from the repository root, through
// bin/factory, as a student would. The command returns once the run has
// started. The commits it makes are counted from here.
func (w *world) start(ctx context.Context, r *runSpec, how invocation) error {
	w.ran = r
	args := []string{"--run", r.name}
	if !w.noTarget {
		args = append(args, "--target", r.target)
	}
	line := r.line
	if how.line != "" {
		line = how.line
	}
	if !how.nameAndTarget {
		args = append(args, "--line", line, "--seed", r.seed)
	}
	if w.attempts > 0 {
		args = append(args, "--attempts", strconv.Itoa(w.attempts))
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
	w.output, w.exitErr = w.factory(ctx, args...)
	w.timedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
	return nil
}

// factory runs bin/factory from the repository root with the arguments, in
// the example's environment, and returns what it said.
func (w *world) factory(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, filepath.Join(w.repo, "bin", "factory"), args...)
	cmd.Dir = w.repo
	cmd.Env = w.environment()
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if w.realPi {
		cmd.Stdout = io.MultiWriter(&out, os.Stdout)
		cmd.Stderr = io.MultiWriter(&out, os.Stderr)
	}
	err := cmd.Run()
	return out.String(), err
}

// environment is what the factory is run in: the suite's, with the
// doubles' folder, and pi-rpc-acp on the PATH as a Codespace has it. Unless
// the machines run the real pi, the double called pi-rpc-acp comes first,
// and no machine signs in with an API key unless an example gives it one.
func (w *world) environment() []string {
	env := append(os.Environ(), "DOUBLE_DIR="+w.doubleDir)
	path := os.Getenv("PATH") + string(os.PathListSeparator) + bridgeDir
	if !w.realPi {
		env = slices.DeleteFunc(env, func(kv string) bool {
			name, _, _ := strings.Cut(kv, "=")
			return strings.HasSuffix(name, "_API_KEY")
		})
		path = fakePiDir + string(os.PathListSeparator) + path
	}
	return append(append(env, "PATH="+path), w.env...)
}

// runToSuccess runs the factory, and fails unless it succeeds.
func (w *world) runToSuccess() error {
	return w.runRunToSuccess(w.current(), invocation{})
}

func (w *world) runRunToSuccess(r *runSpec, how invocation) error {
	if err := w.runRun(r, how); err != nil {
		return err
	}
	if w.exitErr != nil || w.timedOut {
		return fmt.Errorf("the factory did not finish (%v):\n%s", w.exitErr, w.output)
	}
	return nil
}

// call is one call a double was given: the name it was called by, the
// machine it was asked to be, and the name of that machine's folder.
type call struct {
	name, role, machine string
	prompt              string // the machine's AGENTS.md, then the prompt
	signedIn            string // how it signed in: its login, or "api key"
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
		if len(fields) != 3 {
			return nil, fmt.Errorf("calls.log line %d is %q", n+1, line)
		}
		prompt, err := os.ReadFile(filepath.Join(w.doubleDir, "prompts", fmt.Sprintf("%d.txt", n+1)))
		if err != nil {
			return nil, err
		}
		signedIn, err := os.ReadFile(filepath.Join(w.doubleDir, "signins", fmt.Sprintf("%d.txt", n+1)))
		if err != nil {
			return nil, err
		}
		calls = append(calls, call{fields[0], fields[1], fields[2], string(prompt), string(signedIn)})
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

// callsToMachine lists the calls made to the machine in that folder.
func (w *world) callsToMachine(name string) ([]call, error) {
	calls, err := w.calls()
	var mine []call
	for _, c := range calls {
		if c.machine == name {
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

// reportsMatching checks the factory's output has words matching the
// pattern.
func (w *world) reportsMatching(pattern string) error {
	if !regexp.MustCompile(pattern).MatchString(w.output) {
		return fmt.Errorf("expected the factory to report something like %q, but it said:\n%s", pattern, w.output)
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
		return ctx, w.tearDown()
	})
	w.registerSetupSteps(sc)
	w.registerLineSteps(sc)
	w.registerRunSteps(sc)
	w.registerPlanSteps(sc)
	w.registerCommitSteps(sc)
	w.registerTargetSteps(sc)
	w.registerRealAgentSteps(sc)
	w.registerValidationSteps(sc)
	w.registerMachineSteps(sc)
	w.registerFanOutSteps(sc)
	w.registerDaemonSteps(sc)
	w.registerWatchSteps(sc)
}

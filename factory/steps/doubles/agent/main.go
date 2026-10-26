// Command agent is a test double for a coding agent such as pi, run as an
// ACP agent as pi is through pi-rpc-acp: it speaks the Agent Client Protocol
// on its standard input and output. It is started the way the factory starts
// pi-rpc-acp, in the target folder:
//
//	agent --no-context-files --no-skills --model <model> \
//	    --append-system-prompt <AGENTS.md>
//
// with PI_CODING_AGENT_DIR set to the machine's folder, and given its prompt
// in a session/prompt request. It plays the machine its instructions
// describe: the planner, the doer, the validator, or one of the three big
// brains, a reviewer or the synthesiser. Several may run at once, as the
// branches of a fan out do. What it generates it sends as
// agent_message_chunk updates, a line at a time, and it reports what its
// turn spent as usage on its response: a token in for each byte it was
// given, and a token out for each byte it generated.
//
// Like pi, it is given the AGENTS.md named by --append-system-prompt, and
// signs in with the auth.json in PI_CODING_AGENT_DIR or else with an API key
// from the environment; with neither it fails, as pi does.
//
// It keeps everything in the folder named by DOUBLE_DIR:
//   - calls.log: one line per call, the name it was called by (pi-rpc-acp or
//     agent), the machine it played, and the machine's folder's name
//   - prompts/<n>.txt: what call n was given: the AGENTS.md, then the prompt
//   - signins/<n>.txt: how call n signed in: the login it found or "api key"
//   - said/<n>.txt: what call n generated, a line at a time
//   - usage.log: what each call reported it spent, "<n> <machine> <in> <out>"
//   - pids.log: the process of each call, so a step can tell whether any is
//     still running
//   - holds.log: "<n> <machine>" for each call that is part-way through its
//     job, held there while a file hold-<role> (hold-doer, hold-reviewer)
//     is in the folder
//   - behaviour.json: what it does, written by the step definitions
//
// Calls that run at the same time take turns at writing these, holding a
// lock on the folder's "lock" file.
//
// Like a machine told to keep what it makes for the run in the run's folder,
// it keeps a note of each call there.
//
// Without a behaviour it answers {"complete": true} and does nothing.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Behaviour is what the steps tell the double to do.
type Behaviour struct {
	// Tasks are the tasks it plans when there is no plan.
	Tasks []string `json:"tasks"`
	// PlannerProse has the planner keep the plan as sentences rather than a
	// checklist; DoerProse has the doer read it that way.
	PlannerProse bool `json:"planner_prose,omitempty"`
	DoerProse    bool `json:"doer_prose,omitempty"`
	// ProductFile, if set, is the one file it writes for a task. Otherwise it
	// writes <task>.txt.
	ProductFile string `json:"product_file,omitempty"`
	// ValidatorSays is a line the validator says before its result.
	ValidatorSays string `json:"validator_says,omitempty"`
	// ValidatorNoResult makes the validator answer in prose only.
	ValidatorNoResult bool `json:"validator_no_result,omitempty"`
	// Rejects is how many times, from the start, the validator is not
	// satisfied; -1 for always.
	Rejects int `json:"rejects,omitempty"`
	// Finding is what the validator finds wrong when it is not satisfied.
	Finding string `json:"finding,omitempty"`

	// ReviewersWait has no reviewer finish until all three have begun.
	ReviewersWait bool `json:"reviewers_wait,omitempty"`
	// NumberedReports has every reviewer not satisfied, its finding
	// "report <n>", numbered in the order they report.
	NumberedReports bool `json:"numbered_reports,omitempty"`
	// ReviewerFindings has the reviewers in these folders not satisfied, each
	// with its finding.
	ReviewerFindings map[string]string `json:"reviewer_findings,omitempty"`
	// Prose has the machines in these folders answer in prose, with no result.
	Prose []string `json:"prose,omitempty"`
	// SynthesiserRejects is how many times, from the start, the synthesiser
	// is not satisfied; -1 for always. It then finds SynthesiserFinding.
	SynthesiserRejects int    `json:"synthesiser_rejects,omitempty"`
	SynthesiserFinding string `json:"synthesiser_finding,omitempty"`

	// DoerSays are lines the doer says as it begins its attempt, and
	// DoerThenSays lines it says once it carries on to the attempt's end.
	DoerSays     []string `json:"doer_says,omitempty"`
	DoerThenSays []string `json:"doer_then_says,omitempty"`
}

// planPath is where the plan is, as the factory's prompt says: either "The
// plan is <path>", or "There is no plan yet. It goes in <path>".
var planPath string

// planIn finds the plan's path in the prompt.
func planIn(prompt string) string {
	for _, line := range strings.Split(prompt, "\n") {
		for _, prefix := range []string{"The plan is ", "There is no plan yet. It goes in "} {
			if path, ok := strings.CutPrefix(line, prefix); ok {
				return strings.TrimSpace(path)
			}
		}
	}
	fail("the prompt does not say where the plan is:\n%s", prompt)
	return ""
}

var checklistLine = regexp.MustCompile(`^- \[( |x)\] (.+)$`)

func main() {
	serve(play)
}

// play is a turn: the double plays the machine its instructions say it is.
// It fails the turn, as pi does, if it cannot sign in.
func play(prompt string) error {
	dir := os.Getenv("DOUBLE_DIR")
	if dir == "" {
		answer(map[string]any{"complete": true})
		return nil
	}
	role := machineIn(prompt)
	machine := filepath.Base(os.Getenv("PI_CODING_AGENT_DIR"))
	// The synthesiser is given the reviewers' results and nothing else: no
	// plan and no run's folder. Every other machine is told both.
	if role != "synthesiser" {
		planPath = planIn(prompt)
	}
	n, signedIn := logCall(dir, role, machine, prompt)
	if !signedIn {
		return fmt.Errorf("No API key found for %s.\n\nUse /login to log into a provider via OAuth or API key.", provider())
	}
	if role != "synthesiser" {
		keepNote(role, prompt)
	}

	var b Behaviour
	raw, err := os.ReadFile(filepath.Join(dir, "behaviour.json"))
	if err != nil {
		answer(map[string]any{"complete": true})
		return nil
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		fail("reading behaviour: %v", err)
	}
	if slices.Contains(b.Prose, machine) {
		say("I looked at the work and it seems fine.")
		return nil
	}

	switch role {
	case "validator":
		if b.ValidatorSays != "" {
			say(b.ValidatorSays)
		}
		if b.ValidatorNoResult {
			say("I looked at the work and it seems fine.")
			return nil
		}
		answer(validate(b, dir))
	case "reviewer":
		if b.ReviewersWait {
			waitForTheOtherReviewers(dir, n)
		}
		hold(dir, role, machine, n)
		res := review(b, dir, machine)
		if res["satisfied"] == true {
			say("satisfied")
		} else {
			say("not satisfied")
		}
		answer(res)
	case "synthesiser":
		answer(synthesise(b, dir))
	case "planner":
		answer(plan(b))
	default:
		for _, line := range b.DoerSays {
			say(line)
		}
		res := work(b)
		hold(dir, "doer", machine, n)
		for _, line := range b.DoerThenSays {
			say(line)
		}
		answer(res)
	}
	return nil
}

// hold keeps the machine part-way through its job while the double's
// folder holds hold-<role>, noting in holds.log that it is held. It goes
// on when the hold is taken away, and gives up if the factory that called
// it goes away first.
func hold(dir, role, machine string, n int) {
	held := filepath.Join(dir, "hold-"+role)
	if _, err := os.Stat(held); err != nil {
		return
	}
	withLock(dir, func() { appendLine(filepath.Join(dir, "holds.log"), fmt.Sprintf("%d %s", n, machine)) })
	for {
		if _, err := os.Stat(held); err != nil {
			return
		}
		select {
		case <-closed:
			os.Exit(0)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// A reviewer is satisfied unless it is told otherwise.
func review(b Behaviour, dir, machine string) map[string]any {
	if b.NumberedReports {
		var n int
		withLock(dir, func() {
			n = countLines(filepath.Join(dir, "reports.log")) + 1
			appendLine(filepath.Join(dir, "reports.log"), machine)
		})
		return map[string]any{"satisfied": false, "findings": []string{fmt.Sprintf("report %d", n)}}
	}
	if finding, ok := b.ReviewerFindings[machine]; ok {
		return map[string]any{"satisfied": false, "findings": []string{finding}}
	}
	return map[string]any{"satisfied": true, "findings": []string{}}
}

// The synthesiser decides, as the validator does, and reports nothing of
// what the reviewers found.
func synthesise(b Behaviour, dir string) map[string]any {
	if b.SynthesiserRejects < 0 || callsTo(dir, "synthesiser") <= b.SynthesiserRejects {
		return map[string]any{"satisfied": false, "findings": []string{b.SynthesiserFinding}}
	}
	return map[string]any{"satisfied": true, "findings": []string{}}
}

// The three big brains have three reviewers.
const reviewers = 3

// waitForTheOtherReviewers holds the reviewer, the nth call, until all three
// reviewers of its attempt have begun: three reviewer calls for each attempt
// so far. It stops waiting, and fails, if the run finishes first, which it
// knows by the factory that called it closing its input.
func waitForTheOtherReviewers(dir string, n int) {
	mine := 0
	for i, c := range readCalls(dir) {
		if i < n && c.role == "reviewer" {
			mine++
		}
	}
	want := (mine + reviewers - 1) / reviewers * reviewers
	for callsTo(dir, "reviewer") < want {
		select {
		case <-closed:
			fail("the run finished before all three reviewers began")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// validate is satisfied unless it is still to reject the work.
func validate(b Behaviour, dir string) map[string]any {
	if b.Rejects < 0 || callsTo(dir, "validator") <= b.Rejects {
		return map[string]any{"satisfied": false, "findings": []string{b.Finding}}
	}
	return map[string]any{"satisfied": true, "findings": []string{}}
}

// callsTo counts the calls so far, this one included, to the machine in
// that role.
func callsTo(dir, role string) int {
	n := 0
	for _, c := range readCalls(dir) {
		if c.role == role {
			n++
		}
	}
	return n
}

type call struct{ name, role, machine string }

// readCalls reads calls.log, taking its turn with the calls writing to it.
func readCalls(dir string) []call {
	var raw []byte
	var err error
	withLock(dir, func() { raw, err = os.ReadFile(filepath.Join(dir, "calls.log")) })
	if err != nil {
		fail("%v", err)
	}
	var calls []call
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if fields := strings.Fields(line); len(fields) == 3 {
			calls = append(calls, call{fields[0], fields[1], fields[2]})
		}
	}
	return calls
}

// plan, for the planner, writes the plan if there is none. Otherwise it marks
// as done each task whose work the target's history shows was committed, and
// says whether any task is left.
func plan(b Behaviour) map[string]any {
	raw, err := os.ReadFile(planPath)
	if os.IsNotExist(err) {
		writePlan(b.PlannerProse, b.Tasks, nil)
		return map[string]any{"complete": false}
	}
	if err != nil {
		fail("reading the plan: %v", err)
	}

	tasks, done := readPlan(b, b.PlannerProse, string(raw))
	committed := commitMessages()
	complete := true
	for _, task := range tasks {
		if committed[task] {
			done[task] = true
		}
		complete = complete && done[task]
	}
	writePlan(b.PlannerProse, tasks, done)
	return map[string]any{"complete": complete}
}

// commitMessages are the subjects of the commits made in the target (the
// folder it runs in), whatever else the repository holds.
func commitMessages() map[string]bool {
	out, err := exec.Command("git", "log", "--format=%s", "--", ".").Output()
	if err != nil {
		fail("reading the history: %v", err)
	}
	messages := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		messages[line] = true
	}
	return messages
}

// work, for the doer, writes the product of the first task not done, and
// says which task that was.
func work(b Behaviour) map[string]any {
	raw, err := os.ReadFile(planPath)
	if err != nil {
		fail("reading the plan: %v", err)
	}
	tasks, done := readPlan(b, b.DoerProse, string(raw))
	for _, task := range tasks {
		if done[task] {
			continue
		}
		product := b.ProductFile
		if product == "" {
			product = task + ".txt"
		}
		if err := os.WriteFile(product, []byte("work for "+task+"\n"), 0o644); err != nil {
			fail("writing the work: %v", err)
		}
		return map[string]any{"task": task}
	}
	return map[string]any{"task": ""}
}

func readPlan(b Behaviour, prose bool, plan string) ([]string, map[string]bool) {
	done := map[string]bool{}
	if prose {
		for _, task := range b.Tasks {
			done[task] = strings.Contains(plan, "Finished "+task+".")
		}
		return b.Tasks, done
	}
	var tasks []string
	for _, line := range strings.Split(plan, "\n") {
		if m := checklistLine.FindStringSubmatch(line); m != nil {
			tasks = append(tasks, m[2])
			done[m[2]] = m[1] == "x"
		}
	}
	return tasks, done
}

func writePlan(prose bool, tasks []string, done map[string]bool) {
	var plan strings.Builder
	if prose {
		fmt.Fprintf(&plan, "We will do %s, in that order.\n", strings.Join(tasks, ", then "))
		for _, task := range tasks {
			if done[task] {
				fmt.Fprintf(&plan, "Finished %s.\n", task)
			}
		}
	} else {
		plan.WriteString("# Plan\n\n")
		for _, task := range tasks {
			mark := " "
			if done[task] {
				mark = "x"
			}
			fmt.Fprintf(&plan, "- [%s] %s\n", mark, task)
		}
	}
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		fail("making the plan's folder: %v", err)
	}
	if err := os.WriteFile(planPath, []byte(plan.String()), 0o644); err != nil {
		fail("writing the plan: %v", err)
	}
}

// keepNote does what a machine's instructions ask: it keeps anything it makes
// for the run, here a note of its call, in the run's folder the prompt names.
func keepNote(role, prompt string) {
	const prefix = "The run's folder, where you keep anything you make for the run, is "
	for _, line := range strings.Split(prompt, "\n") {
		if folder, ok := strings.CutPrefix(line, prefix); ok {
			notes, err := os.OpenFile(filepath.Join(strings.TrimSpace(folder), role+"-notes.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				fail("keeping a note: %v", err)
			}
			defer notes.Close()
			fmt.Fprintln(notes, "called")
			return
		}
	}
	fail("the prompt does not say where the run's folder is:\n%s", prompt)
}

// machineIn is the machine being played, as a real agent would know it: by
// its instructions. The first line of the AGENTS.md it is given, "# Planner",
// "# Doer" or "# Validator", says what it is for, whatever its folder is
// called. Without instructions, it is the machine its folder is named for.
func machineIn(_ string) string {
	first, _, _ := strings.Cut(strings.TrimSpace(appendedInstructions()), "\n")
	if heading, ok := strings.CutPrefix(first, "# "); ok {
		return strings.ToLower(strings.TrimSpace(heading))
	}
	if name := filepath.Base(os.Getenv("PI_CODING_AGENT_DIR")); name != "." {
		return name
	}
	return "unknown"
}

// logCall records the call: the name it was called by, its role and its
// machine's folder, what it was given (the instructions in the AGENTS.md it
// was pointed at, then the prompt), how it signed in and its process. It
// returns the call's number, and whether it signed in.
func logCall(dir, role, machine, prompt string) (int, bool) {
	given := prompt
	if instructions := appendedInstructions(); instructions != "" {
		given = instructions + "\n---\n" + prompt
	}
	how := signIn()
	spentIn = len(given)
	var n int
	withLock(dir, func() {
		for _, sub := range []string{"prompts", "signins", "said"} {
			if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
				fail("%v", err)
			}
		}
		n = countLines(filepath.Join(dir, "calls.log")) + 1
		if err := os.WriteFile(filepath.Join(dir, "prompts", fmt.Sprintf("%d.txt", n)), []byte(given), 0o644); err != nil {
			fail("%v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "signins", fmt.Sprintf("%d.txt", n)), []byte(how), 0o644); err != nil {
			fail("%v", err)
		}
		appendLine(filepath.Join(dir, "pids.log"), strconv.Itoa(os.Getpid()))
		appendLine(filepath.Join(dir, "calls.log"), fmt.Sprintf("%s %s %s", filepath.Base(os.Args[0]), role, machine))
	})
	callNumber = n
	return n, how != ""
}

// withLock runs f holding the lock on the double's folder, so that calls
// running at the same time take turns.
func withLock(dir string, f func()) {
	lock, err := os.OpenFile(filepath.Join(dir, "lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		fail("%v", err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		fail("locking: %v", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	f()
}

func countLines(path string) int {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		fail("%v", err)
	}
	return strings.Count(string(raw), "\n")
}

func appendLine(path, line string) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fail("%v", err)
	}
	defer file.Close()
	fmt.Fprintln(file, line)
}

// appendedInstructions are the contents of the file named by
// --append-system-prompt, if the double was given one.
func appendedInstructions() string {
	for i, arg := range os.Args {
		if arg == "--append-system-prompt" && i+1 < len(os.Args) {
			raw, err := os.ReadFile(os.Args[i+1])
			if err != nil {
				fail("reading %s: %v", os.Args[i+1], err)
			}
			return string(raw)
		}
	}
	return ""
}

// signIn is how this call signs in, as pi would: with the login in its
// configuration folder, else with an API key; "" with neither.
func signIn() string {
	if login, err := os.ReadFile(filepath.Join(os.Getenv("PI_CODING_AGENT_DIR"), "auth.json")); err == nil {
		return "login " + strings.TrimSpace(string(login))
	}
	if hasAPIKey() {
		return "api key"
	}
	return ""
}

func hasAPIKey() bool {
	for _, kv := range os.Environ() {
		if name, value, _ := strings.Cut(kv, "="); strings.HasSuffix(name, "_API_KEY") && value != "" {
			return true
		}
	}
	return false
}

func answer(result map[string]any) {
	line, _ := json.Marshal(result)
	say(string(line))
}

// provider is the provider of the model it was given.
func provider() string {
	for i, arg := range os.Args {
		if arg == "--model" && i+1 < len(os.Args) {
			p, _, _ := strings.Cut(os.Args[i+1], "/")
			return p
		}
	}
	return "its provider"
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "agent double: "+format+"\n", args...)
	os.Exit(1)
}

// Command agent is a test double for a coding agent such as pi. It is called
// the way the factory calls pi, in the target folder:
//
//	agent -p --no-context-files --no-skills --model <model> \
//	    --append-system-prompt <AGENTS.md> <prompt>
//
// with PI_CODING_AGENT_DIR set to the machine's folder. It plays the machine
// its instructions describe: the planner, the doer or the validator.
//
// Like pi, it is given the AGENTS.md named by --append-system-prompt, and
// signs in with the auth.json in PI_CODING_AGENT_DIR or else with an API key
// from the environment; with neither it fails, as pi does.
//
// It keeps everything in the folder named by DOUBLE_DIR:
//   - calls.log: one line per call, the name it was called by (pi or agent)
//     and the machine it played
//   - prompts/<n>.txt: what call n was given: the AGENTS.md, then the prompt
//   - signins/<n>.txt: how call n signed in: the login it found or "api key"
//   - behaviour.json: what it does, written by the step definitions
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
	"strings"
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
	dir := os.Getenv("DOUBLE_DIR")
	if dir == "" {
		answer(map[string]any{"complete": true})
		return
	}
	prompt := os.Args[len(os.Args)-1]
	role := machineIn(prompt)
	planPath = planIn(prompt)
	logCall(dir, role, prompt)
	signIn(dir)
	keepNote(role, prompt)

	var b Behaviour
	raw, err := os.ReadFile(filepath.Join(dir, "behaviour.json"))
	if err != nil {
		answer(map[string]any{"complete": true})
		return
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		fail("reading behaviour: %v", err)
	}

	switch role {
	case "validator":
		if b.ValidatorSays != "" {
			fmt.Println(b.ValidatorSays)
		}
		if b.ValidatorNoResult {
			fmt.Println("I looked at the work and it seems fine.")
			return
		}
		answer(validate(b, dir))
	case "planner":
		answer(plan(b))
	default:
		answer(work(b))
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
	raw, err := os.ReadFile(filepath.Join(dir, "calls.log"))
	if err != nil {
		fail("%v", err)
	}
	n := 0
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasSuffix(line, " "+role) {
			n++
		}
	}
	return n
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

// logCall records the call's name and role, and what it was given: the
// instructions in the AGENTS.md it was pointed at, then the prompt.
func logCall(dir, role, prompt string) {
	prompts := filepath.Join(dir, "prompts")
	if err := os.MkdirAll(prompts, 0o755); err != nil {
		fail("%v", err)
	}
	entries, _ := os.ReadDir(prompts)
	n := len(entries) + 1
	given := prompt
	if instructions := appendedInstructions(); instructions != "" {
		given = instructions + "\n---\n" + prompt
	}
	if err := os.WriteFile(filepath.Join(prompts, fmt.Sprintf("%d.txt", n)), []byte(given), 0o644); err != nil {
		fail("%v", err)
	}
	log, err := os.OpenFile(filepath.Join(dir, "calls.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fail("%v", err)
	}
	defer log.Close()
	fmt.Fprintln(log, filepath.Base(os.Args[0]), role)
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

// signIn notes how this call signs in, as pi would: with the login in its
// configuration folder, else with an API key. With neither it fails.
func signIn(dir string) {
	signins := filepath.Join(dir, "signins")
	if err := os.MkdirAll(signins, 0o755); err != nil {
		fail("%v", err)
	}
	entries, _ := os.ReadDir(signins)
	how := ""
	if login, err := os.ReadFile(filepath.Join(os.Getenv("PI_CODING_AGENT_DIR"), "auth.json")); err == nil {
		how = "login " + strings.TrimSpace(string(login))
	} else if hasAPIKey() {
		how = "api key"
	}
	if err := os.WriteFile(filepath.Join(signins, fmt.Sprintf("%d.txt", len(entries)+1)), []byte(how), 0o644); err != nil {
		fail("%v", err)
	}
	if how == "" {
		fmt.Fprintln(os.Stderr, "No API key found. Use /login to log into a provider.")
		os.Exit(1)
	}
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
	fmt.Println(string(line))
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "agent double: "+format+"\n", args...)
	os.Exit(1)
}

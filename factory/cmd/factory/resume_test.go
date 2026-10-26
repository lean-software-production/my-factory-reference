package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/lean-software-production/my-factory-reference/factory/internal/acptest"
)

// A run that is interrupted, as when the Codespace sleeps or the factory is
// killed outright, carries on from its plan when it is started again. That
// needs the plan to be in Git by then: it is committed as soon as the
// planner has answered, before the doer is called, so the doer's work and
// the plan are never the only record of the run's progress.
func TestARunInterruptedAfterThePlannersCallHasThePlanCommitted(t *testing.T) {
	binary := buildFactory(t)
	state, target := t.TempDir(), t.TempDir()
	line := filepath.Join(target, ".assembly-lines", "loop")
	plan := filepath.Join(line, "runs", "r", "plan.md")

	// The planner writes the plan if there is none, and notes what it was
	// told. The doer says it has begun and stays until it is let go.
	planner := acptest.Script(`printf '%s\n' "$request" >> "`+state+`/prompts"
[ -f "`+plan+`" ] || printf -- '- [ ] one\n- [ ] two\n' > "`+plan+`"`, `{"complete": false, "progress": {"done": 0, "total": 2}}`)
	doer := acptest.Script(`echo $$ >> "`+state+`/pids"
echo begun >> "`+state+`/begun"
while [ ! -f "`+state+`/go" ]; do sleep 0.01; done`, `{"task": "one"}`)
	writeLine(t, line, map[string]string{"planner": planner, "doer": doer})
	if err := os.WriteFile(filepath.Join(target, "seed.md"), []byte("Build a game.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.WriteFile(filepath.Join(state, "go"), nil, 0o644) }) // however the test ends
	start := func() {
		runFactory(t, binary, target, "--run", "r", "--line", "loop", "--seed", filepath.Join(target, "seed.md"), "--target", target)
	}

	start()
	acptest.WaitForFile(t, filepath.Join(state, "begun"))
	if err := syscall.Kill(daemonPid(t, binary), syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}

	// The doer had begun, so the planner's call was over: its plan is in Git.
	if log, err := git(target, "log", "--format=%s", "--", plan); err != nil || !strings.Contains(log, "Write the plan") {
		t.Errorf("the plan was not committed after the planner's call (%v):\n%s", err, log)
	}
	if committed, err := git(target, "show", "HEAD:"+strings.TrimPrefix(plan, target+"/")); err != nil || !strings.Contains(committed, "- [ ] two") {
		t.Errorf("the committed plan is not the planner's (%v):\n%s", err, committed)
	}

	// Started again, the run resumes: the planner is given the plan that
	// exists, not told there is none, and the daemon that was killed does
	// not stand in the way.
	start()
	if !waitForLines(t, filepath.Join(state, "prompts"), 2) {
		t.Fatal("the planner was not called again when the run was started again")
	}
	said, _ := os.ReadFile(filepath.Join(state, "prompts"))
	calls := strings.Split(strings.TrimSpace(string(said)), "\n")
	if !strings.Contains(calls[0], "no plan yet") {
		t.Errorf("the first call was not told there was no plan:\n%s", calls[0])
	}
	if strings.Contains(calls[1], "no plan yet") || !strings.Contains(calls[1], "The plan is "+plan) {
		t.Errorf("the planner, called again, was not given the plan:\n%s", calls[1])
	}

	os.WriteFile(filepath.Join(state, "go"), nil, 0o644)
	runFactory(t, binary, "", "stop")
	acptest.NoneRunning(t, state)
}

// writeLine writes a line of a planner and a doer, each run by the harness
// script given for it, with a login each.
func writeLine(t *testing.T, line string, harnesses map[string]string) {
	t.Helper()
	graph := `digraph { start -> planner; planner -> doer [label="not complete"]; planner -> finish [label=complete]; doer -> planner }`
	if err := os.MkdirAll(line, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(line, "assembly-line.dot"), []byte(graph), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, script := range harnesses {
		dir := filepath.Join(line, "machines", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		harness := acptest.Harness(t, name+"-harness", script)
		for file, text := range map[string]string{
			"machine.json": `{"harness": "` + harness + `", "model": "anthropic/test"}`,
			"AGENTS.md":    "# " + name + "\n",
			"auth.json":    `{"anthropic": {}}`,
		} {
			if err := os.WriteFile(filepath.Join(dir, file), []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// waitForLines waits until the file has at least n lines, and says whether it did.
func waitForLines(t *testing.T, path string, n int) bool {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if raw, _ := os.ReadFile(path); strings.Count(string(raw), "\n") >= n {
			return true
		}
	}
	return false
}

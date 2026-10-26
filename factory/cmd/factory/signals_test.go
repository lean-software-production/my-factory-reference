package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/lean-software-production/my-factory-reference/factory/internal/acptest"
)

// The factory started with hangups ignored, as nohup starts it, carries on
// when it is hung up on; started otherwise, it stops, and stops its run and
// the run's machine with it.
func TestAHangupStopsARunUnlessHangupsAreIgnored(t *testing.T) {
	for name, ignored := range map[string]bool{"ignored, as nohup does": true, "from a terminal": false} {
		t.Run(name, func(t *testing.T) {
			binary := buildFactory(t)
			state, target := t.TempDir(), oneMachineTarget(t)
			// The planner says it has begun, then waits until it is let go.
			planner := acptest.Script(`echo $$ > "`+state+`/planner"
while [ ! -f "`+state+`/go" ]; do sleep 0.01; done`, `{"complete": true}`)
			if err := os.WriteFile(filepath.Join(target, "harness"), []byte(planner), 0o755); err != nil {
				t.Fatal(err)
			}
			shell := `exec "$0" "$@"`
			if ignored {
				shell = `trap "" HUP; ` + shell
			}
			start := exec.Command("sh", "-c", shell, binary, "--run", "r", "--line", "only", "--seed", filepath.Join(target, "seed.md"), "--target", target)
			if out, err := start.CombinedOutput(); err != nil {
				t.Fatalf("starting the run: %v\n%s", err, out)
			}
			acptest.WaitForFile(t, filepath.Join(state, "planner"))
			raw, _ := os.ReadFile(filepath.Join(state, "planner"))
			plannerPid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
			daemon := daemonPid(t, binary)

			if err := syscall.Kill(daemon, syscall.SIGHUP); err != nil {
				t.Fatal(err)
			}
			if !ignored {
				if !waitUntilGone(daemon, 10*time.Second) {
					t.Fatal("the factory was still running 10s after a hangup")
				}
				for deadline := time.Now().Add(2 * time.Second); !acptest.ProcessGone(plannerPid) && time.Now().Before(deadline); {
					time.Sleep(10 * time.Millisecond)
				}
				if !acptest.ProcessGone(plannerPid) {
					syscall.Kill(plannerPid, syscall.SIGKILL)
					t.Error("the planner was still running after the factory stopped")
				}
				return
			}
			// Ignored, the hangup changes nothing: the planner is still at
			// work a second later, and the run ends when it answers.
			time.Sleep(time.Second)
			if acptest.ProcessGone(plannerPid) {
				t.Errorf("the planner was stopped by a hangup the factory ignores")
			}
			if err := os.WriteFile(filepath.Join(state, "go"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
			out := runFactory(t, binary, target, "watch", "r", "--target", target)
			if !strings.Contains(out, "run r finished") {
				t.Errorf("the run did not finish after an ignored hangup:\n%s", out)
			}
		})
	}
}

var runningProcess = regexp.MustCompile(`the factory is running \(process (\d+)\)`)

// daemonPid is the process of the factory's daemon, as it says it is.
func daemonPid(t *testing.T, binary string) int {
	t.Helper()
	out := runFactory(t, binary, "", "status")
	m := runningProcess.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("the factory is not running:\n%s", out)
	}
	pid, _ := strconv.Atoi(m[1])
	return pid
}

// oneMachineTarget is a target with a seed and a line, "only", on which the
// planner, whose harness is the target's "harness", is the only machine.
func oneMachineTarget(t *testing.T) string {
	t.Helper()
	target := t.TempDir()
	machine := filepath.Join(target, ".assembly-lines", "only", "machines", "planner")
	if err := os.MkdirAll(machine, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, text := range map[string]string{
		filepath.Join(target, "seed.md"):                                      "Build a game.\n",
		filepath.Join(target, ".assembly-lines", "only", "assembly-line.dot"): `digraph { start -> planner; planner -> finish [label=complete] }`,
		filepath.Join(machine, "machine.json"):                                `{"harness": "` + filepath.Join(target, "harness") + `", "model": "anthropic/test"}`,
		filepath.Join(machine, "AGENTS.md"):                                   "# Planner\n",
		filepath.Join(machine, "auth.json"):                                   `{"anthropic": {}}`,
	} {
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return target
}

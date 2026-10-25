package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A run started with hangups ignored, as nohup starts it, carries on when
// its terminal closes; one started from a terminal stops, and stops its
// machine with it.
func TestAHangupStopsARunUnlessHangupsAreIgnored(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "factory")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("building the factory: %v\n%s", err, out)
	}
	for name, ignored := range map[string]bool{"ignored, as nohup does": true, "from a terminal": false} {
		t.Run(name, func(t *testing.T) {
			state, target := t.TempDir(), oneMachineTarget(t)
			// The planner says it has begun, then waits until it is let go.
			planner := `#!/bin/sh
echo $$ > "` + state + `/planner"
while [ ! -f "` + state + `/go" ]; do sleep 0.01; done
echo '{"complete": true}'
`
			if err := os.WriteFile(filepath.Join(target, "harness"), []byte(planner), 0o755); err != nil {
				t.Fatal(err)
			}
			shell := `exec "$0" "$@"`
			if ignored {
				shell = `trap "" HUP; ` + shell
			}
			factory := exec.Command("sh", "-c", shell, binary, "--run", "r", "--line", "only", "--seed", filepath.Join(target, "seed.md"), "--target", target)
			var out strings.Builder
			factory.Stdout, factory.Stderr = &out, &out
			if err := factory.Start(); err != nil {
				t.Fatal(err)
			}
			exited := make(chan error, 1)
			go func() { exited <- factory.Wait() }()
			waitForFile(t, filepath.Join(state, "planner"))
			raw, _ := os.ReadFile(filepath.Join(state, "planner"))
			plannerPid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))

			if err := factory.Process.Signal(syscall.SIGHUP); err != nil {
				t.Fatal(err)
			}
			if !ignored {
				select {
				case err := <-exited:
					if err == nil {
						t.Errorf("the factory carried on after a hangup:\n%s", out.String())
					}
				case <-time.After(10 * time.Second):
					t.Fatal("the factory was still running 10s after a hangup")
				}
				for deadline := time.Now().Add(2 * time.Second); !processGone(plannerPid) && time.Now().Before(deadline); {
					time.Sleep(10 * time.Millisecond)
				}
				if !processGone(plannerPid) {
					syscall.Kill(plannerPid, syscall.SIGKILL)
					t.Error("the planner was still running after the factory stopped")
				}
				return
			}
			// Ignored, the hangup changes nothing: the planner is still at
			// work a second later, and the run ends when it answers.
			time.Sleep(time.Second)
			if processGone(plannerPid) {
				t.Errorf("the planner was stopped by a hangup the factory ignores:\n%s", out.String())
			}
			if err := os.WriteFile(filepath.Join(state, "go"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-exited:
				if err != nil {
					t.Errorf("the run failed after an ignored hangup (%v):\n%s", err, out.String())
				}
			case <-time.After(10 * time.Second):
				t.Fatal("the run did not end 10s after its planner answered")
			}
		})
	}
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

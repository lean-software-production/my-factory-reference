package machine

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lean-software-production/my-factory-reference/factory/internal/acptest"
)

// A harness runs on the machine's folder and no other configuration: not
// yours, and none found in or above the target. The doubles cannot see
// whether the real pi would find any, so this checks how it is started:
// pi-rpc-acp passes its arguments on to pi.
func TestHarnessIsCalledOnTheMachinesConfigurationOnly(t *testing.T) {
	m := Machine{Name: "doer", Dir: "/factory/machines/doer", Harness: "pi-rpc-acp", Model: "openai-codex/gpt-6-sol"}
	cmd := m.command("/work/target", []string{"PATH=/no/such/folder"})

	if cmd.Dir != "/work/target" {
		t.Errorf("it runs in %s, not in the target", cmd.Dir)
	}
	if !slices.Contains(cmd.Env, "PI_CODING_AGENT_DIR=/factory/machines/doer") {
		t.Errorf("pi is not pointed at the machine's folder: %v", cmd.Env)
	}
	want := []string{"pi-rpc-acp", "--no-context-files", "--no-skills", "--model", "openai-codex/gpt-6-sol",
		"--append-system-prompt", "/factory/machines/doer/AGENTS.md"}
	if !slices.Equal(cmd.Args, want) {
		t.Errorf("pi is called as %q, not as %q", cmd.Args, want)
	}
	if cmd.Err == nil {
		t.Errorf("a harness that is not on the caller's PATH was found, at %s", cmd.Path)
	}
}

// In a Codespace where pi isn't signed in, a machine's harness gives up at
// once. The run must fail within seconds, saying which machine and which
// harness, and why, without hanging and without showing the protocol.
func TestUnsignedHarnessFailsFast(t *testing.T) {
	harness := acptest.Harness(t, "pi-rpc-acp", "#!/bin/sh\necho 'not authenticated' >&2\nexit 1\n")
	// The machine's folder is empty: no login, no configuration.
	m := Machine{Name: "doer", Dir: t.TempDir(), Harness: harness, Model: "openai-codex/gpt-6-sol"}

	const limit = 10 * time.Second
	failed := make(chan error, 1)
	began := time.Now()
	go func() {
		_, err := m.Run(context.Background(), t.TempDir(), "do it", nil, nil, nil)
		failed <- err
	}()
	var err error
	select {
	case err = <-failed:
	case <-time.After(limit):
		t.Fatalf("the machine was still running %v after its harness gave up", limit)
	}
	if err == nil {
		t.Fatal("a machine whose harness is not signed in did its job")
	}
	t.Logf("failed in %v: %v", time.Since(began).Round(time.Millisecond), err)
	for _, want := range []string{"doer", "pi", "not authenticated"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not say %q: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "jsonrpc") {
		t.Errorf("the error shows the protocol: %v", err)
	}
}

// pi-rpc-acp ends a turn pi could not take, because its login is no
// good, as if pi had taken it and said nothing. That fails too, at once,
// saying which machine and harness, and what the harness complained of.
func TestAHarnessThatSaysNothingFailsFast(t *testing.T) {
	m := harnessed(t, "planner", acptest.Script("echo 'No API key for provider: openai-codex' >&2"))
	_, err := m.Run(context.Background(), t.TempDir(), "do it", nil, nil, nil)
	if err == nil {
		t.Fatal("a machine that generated nothing did its job")
	}
	for _, want := range []string{"planner", m.Harness, "generated nothing", "No API key"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not say %q: %v", want, err)
		}
	}
}

// What a machine generates is passed on a line at a time, as it comes, and
// what it answers is the whole of it, with what its turn spent.
func TestAMachineSaysWhatItGeneratesLineByLine(t *testing.T) {
	m := harnessed(t, "doer", acptest.Script("", "working on it", `{"task": "alpha"}`))
	var lines []string
	turn, err := m.Run(context.Background(), t.TempDir(), "do it", nil, func(line string) { lines = append(lines, line) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"working on it", `{"task": "alpha"}`}; !slices.Equal(lines, want) {
		t.Errorf("the machine said %q, not %q", lines, want)
	}
	if turn.Text != "working on it\n{\"task\": \"alpha\"}\n" {
		t.Errorf("the machine answered %q", turn.Text)
	}
	if turn.Usage != (Usage{InputTokens: 10, OutputTokens: 2}) {
		t.Errorf("the turn spent %+v", turn.Usage)
	}
}

// Stopping the factory stops the machine it is running, and whatever the
// machine started, in its process group or, for a harness that cleans up
// when asked to stop as pi does, beyond it. One that won't stop when asked
// is killed.
func TestStoppingTheFactoryStopsItsMachine(t *testing.T) {
	acptest.NeedSetsid(t)
	for name, work := range map[string]string{"its group": acptest.Slow, "like pi": acptest.PiLike, "deaf to SIGTERM": acptest.Stubborn} {
		t.Run(name, func(t *testing.T) {
			state := t.TempDir()
			t.Setenv("FANOUT_STATE", state)
			m := harnessed(t, "slow", acptest.Script(work))
			ctx, stop := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() {
				_, err := m.Run(ctx, t.TempDir(), "do it", nil, nil, nil)
				done <- err
			}()
			acptest.WaitForFile(t, filepath.Join(state, "begun"))
			stop()
			select {
			case err := <-done:
				if err == nil || !strings.Contains(err.Error(), "was stopped") {
					t.Errorf("a stopped machine returned %v", err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("the machine was still running 10s after the factory stopped it")
			}
			acptest.NoneRunning(t, state)
		})
	}
}

// A machine that answers leaves nothing running, and what it left running
// does not keep the factory waiting.
func TestAMachineLeavesNothingRunning(t *testing.T) {
	state := t.TempDir()
	t.Setenv("FANOUT_STATE", state)
	m := harnessed(t, "quick", acptest.Script(acptest.LeavesAChild, `{"satisfied": true}`))
	done := make(chan struct{})
	var turn Turn
	var err error
	go func() {
		defer close(done)
		turn, err = m.Run(context.Background(), t.TempDir(), "do it", nil, nil, nil)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("waited 5s for the machine's call to return")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(turn.Text, `"satisfied"`) {
		t.Errorf("the machine answered %q", turn.Text)
	}
	acptest.NoneRunning(t, state)
}

// harnessed is a machine whose harness is the script, in a folder of its own.
func harnessed(t *testing.T, name, script string) Machine {
	t.Helper()
	return Machine{Name: name, Dir: t.TempDir(), Harness: acptest.Harness(t, "harness", script), Model: "p/m"}
}

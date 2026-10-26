package machine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lean-software-production/my-factory-reference/factory/internal/acptest"
)

// A harness that is not an ACP agent, as plain pi is not, never answers the
// factory's first requests. The machine fails within a deadline, saying
// which machine and harness, and leaves nothing of it running.
func TestAHarnessThatNeverAnswersFailsWithinTheDeadline(t *testing.T) {
	answersInitialize := "read -r request\nprintf '%s\\n' '{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"protocolVersion\":1}}'\n"
	for request, before := range map[string]string{"initialize": "", "session/new": answersInitialize} {
		t.Run(request, func(t *testing.T) {
			defer func(was time.Duration) { startDeadline = was }(startDeadline)
			startDeadline = 300 * time.Millisecond
			state := t.TempDir()
			t.Setenv("FANOUT_STATE", state)
			m := harnessed(t, "doer", "#!/bin/sh\n"+before+acptest.Slow+"\n")

			failed := make(chan error, 1)
			began := time.Now()
			go func() {
				_, err := m.Run(context.Background(), t.TempDir(), "do it", nil, nil, nil)
				failed <- err
			}()
			var err error
			select {
			case err = <-failed:
			case <-time.After(10 * time.Second):
				t.Fatal("the machine was still waiting 10s after its deadline")
			}
			t.Logf("failed in %v: %v", time.Since(began).Round(time.Millisecond), err)
			if err == nil {
				t.Fatal("a machine whose harness never answered did its job")
			}
			for _, want := range []string{"doer", m.Harness, request} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the error does not say %q: %v", want, err)
				}
			}
			acptest.NoneRunning(t, state)
		})
	}
}

// The deadline is on starting the agent, not on its turn, which takes as
// long as it takes.
func TestATurnMayTakeLongerThanTheDeadline(t *testing.T) {
	defer func(was time.Duration) { startDeadline = was }(startDeadline)
	startDeadline = 300 * time.Millisecond
	m := harnessed(t, "doer", acptest.Script("sleep 1", `{"task": "slow"}`))
	turn, err := m.Run(context.Background(), t.TempDir(), "do it", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(turn.Text, `"slow"`) {
		t.Errorf("the machine answered %q", turn.Text)
	}
}

// Plain pi is not an ACP agent: a machine.json that names it is refused,
// pointing at its bridge.
func TestAMachineWhoseHarnessIsPlainPiIsRefused(t *testing.T) {
	for _, harness := range []string{"pi", "/usr/local/bin/pi"} {
		dir := t.TempDir()
		config := `{"harness": "` + harness + `", "model": "openai-codex/gpt-6-sol"}`
		if err := os.WriteFile(filepath.Join(dir, "machine.json"), []byte(config), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := Load("planner", dir)
		if err == nil {
			t.Fatalf("a machine whose harness is %s was loaded", harness)
		}
		if want := "the planner's harness pi is not an ACP agent: use pi-rpc-acp"; err.Error() != want {
			t.Errorf("it was refused with %q, not %q", err, want)
		}
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "machine.json"), []byte(`{"harness": "pi-rpc-acp", "model": "openai-codex/gpt-6-sol"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("planner", dir); err != nil {
		t.Errorf("a machine run by pi-rpc-acp was refused: %v", err)
	}
}

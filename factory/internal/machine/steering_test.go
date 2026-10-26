package machine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lean-software-production/my-factory-reference/factory/internal/acptest"
)

// steered is work for a harness that, once it has its prompt, says it has
// begun, then reads the next request, the message it is told, keeps it in
// $STEER_STATE/steer, and answers it with the reply, if there is one.
func steered(reply string) string {
	work := `echo begun > "$STEER_STATE/begun"
read -r steer
printf '%s\n' "$steer" > "$STEER_STATE/steer"
`
	if reply != "" {
		work += "printf '%s\\n' '" + reply + "'\n"
	}
	return work
}

// tellWhileItWorks runs the machine, and tells it the words once it has
// begun its turn. It returns the turn, and what Tell returned.
func tellWhileItWorks(t *testing.T, script, words string) (Turn, error) {
	t.Helper()
	state := t.TempDir()
	t.Setenv("STEER_STATE", state)
	m := harnessed(t, "doer", script)
	steering := NewSteering()
	type ran struct {
		turn Turn
		err  error
	}
	done := make(chan ran, 1)
	go func() {
		turn, err := m.Run(context.Background(), t.TempDir(), "do it", nil, nil, steering)
		done <- ran{turn, err}
	}()
	acptest.WaitForFile(t, filepath.Join(state, "begun"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	told := steering.Tell(ctx, words)
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("the machine's turn failed: %v", r.err)
		}
		return r.turn, told
	case <-time.After(10 * time.Second):
		t.Fatal("the machine's turn did not end")
	}
	return Turn{}, nil
}

// A machine told something while it works is passed it with ACP's
// _session/steering, in its session, and the factory waits until its agent
// says the message went in.
func TestAMachineIsToldSomethingWhileItWorks(t *testing.T) {
	script := acptest.Script(steered(`{"jsonrpc":"2.0","id":4,"result":{"outcome":"injected"}}`), "heard it", `{"task": "alpha"}`)
	turn, err := tellWhileItWorks(t, script, "use curses, not print")
	if err != nil {
		t.Fatalf("telling the machine failed: %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(os.Getenv("STEER_STATE"), "steer"))
	for _, want := range []string{`"method":"_session/steering"`, `"sessionId":"s"`, `"text":"use curses, not print"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the machine was sent %s, without %s", raw, want)
		}
	}
	if !strings.Contains(turn.Text, `{"task": "alpha"}`) {
		t.Errorf("the turn answered %q", turn.Text)
	}
}

// A machine whose agent will not take the message has not heard it.
func TestAMessageTheAgentWillNotTakeIsRefused(t *testing.T) {
	for name, c := range map[string]struct{ reply, says string }{
		"no turn running": {`{"jsonrpc":"2.0","id":4,"result":{"outcome":"promptRequired","reason":"noRunningTurn"}}`, "noRunningTurn"},
		"no steering":     {`{"jsonrpc":"2.0","id":4,"error":{"code":-32601,"message":"Method not found: _session/steering"}}`, "Method not found"},
		"no answer":       {"", "turn ended"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := tellWhileItWorks(t, acptest.Script(steered(c.reply), `{"task": "alpha"}`), "use curses, not print")
			if err == nil {
				t.Fatal("the machine was told, and its agent did not take it")
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Errorf("the refusal does not say %q: %v", c.says, err)
			}
		})
	}
}

// A machine whose turn is over is told nothing: a message is for a turn
// that is running, and is not kept for the next.
func TestAMachineWhoseTurnIsOverIsToldNothing(t *testing.T) {
	m := harnessed(t, "doer", acptest.Script("", `{"task": "alpha"}`))
	steering := NewSteering()
	if _, err := m.Run(context.Background(), t.TempDir(), "do it", nil, nil, steering); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := steering.Tell(ctx, "too late"); !errors.Is(err, ErrNotWorking) {
		t.Errorf("a machine whose turn was over was told: %v", err)
	}
}

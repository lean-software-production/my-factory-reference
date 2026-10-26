// Package acptest makes harnesses for the factory's tests: shell scripts that
// speak just enough of the Agent Client Protocol to stand in for a machine,
// and checks that what they started has gone.
package acptest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Script is a harness that answers the factory's first three requests in
// turn, initialize, session/new and session/prompt: it runs work, a piece of
// shell, once it has the prompt, then generates the lines and ends its turn.
// It stays until its input closes, as an ACP agent does.
func Script(work string, lines ...string) string {
	var script strings.Builder
	script.WriteString("#!/bin/sh\n")
	reply := func(id int, result any) {
		fmt.Fprintf(&script, "read -r request\nprintf '%%s\\n' %s\n", quote(jsonLine(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})))
	}
	reply(1, map[string]any{"protocolVersion": 1})
	reply(2, map[string]any{"sessionId": "s"})
	script.WriteString("read -r request\n")
	script.WriteString(work + "\n")
	for _, line := range lines {
		fmt.Fprintf(&script, "printf '%%s\\n' %s\n", quote(jsonLine(map[string]any{"jsonrpc": "2.0", "method": "session/update",
			"params": map[string]any{"sessionId": "s", "update": map[string]any{"sessionUpdate": "agent_message_chunk",
				"content": map[string]any{"type": "text", "text": line + "\n"}}}})))
	}
	fmt.Fprintf(&script, "printf '%%s\\n' %s\n", quote(jsonLine(map[string]any{"jsonrpc": "2.0", "id": 3,
		"result": map[string]any{"stopReason": "end_turn", "usage": map[string]any{"inputTokens": 10, "outputTokens": 2}}})))
	script.WriteString("cat > /dev/null\n")
	return script.String()
}

func jsonLine(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// Harness writes the script as an executable called name in a folder of
// its own, and returns its path.
func Harness(t *testing.T, name, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// WaitForFile waits until the file exists.
func WaitForFile(t *testing.T, path string) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(path); err == nil {
			return
		}
	}
	t.Fatalf("%s never appeared", path)
}

// NoneRunning checks every process noted in the state's pids has gone.
func NoneRunning(t *testing.T, state string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(state, "pids"))
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range strings.Fields(string(raw)) {
		pid, err := strconv.Atoi(field)
		if err != nil {
			t.Fatal(err)
		}
		for deadline := time.Now().Add(2 * time.Second); !ProcessGone(pid) && time.Now().Before(deadline); {
			time.Sleep(10 * time.Millisecond)
		}
		if !ProcessGone(pid) {
			ps, _ := exec.Command("ps", "-o", "pid,ppid,pgid,sid,stat,args", "-p", strconv.Itoa(pid)).CombinedOutput()
			syscall.Kill(pid, syscall.SIGKILL)
			t.Errorf("process %d was still running:\n%s", pid, ps)
		}
	}
}

// ProcessGone says whether the process has gone. One that has exited and
// that another parent has yet to reap has gone too, where the system says
// so (Linux's /proc); and so has one /proc no longer lists, though a signal
// reached it a moment ago.
func ProcessGone(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return true
	}
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		_, noProc := os.Stat("/proc/self")
		return noProc == nil
	}
	_, after, _ := strings.Cut(string(stat), ") ")
	return strings.HasPrefix(after, "Z")
}

// NeedSetsid skips a test whose harness needs setsid, which not every
// system has (macOS doesn't).
func NeedSetsid(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("setsid"); err != nil {
		t.Skip("setsid is not installed")
	}
}

// Work for harnesses that stand in for machines being stopped. Each notes
// its process, and that of a child it starts, in $FANOUT_STATE/pids, and
// that it has begun in $FANOUT_STATE/begun.
const (
	// Slow keeps working, child and all, until it is stopped.
	Slow = `echo $$ >> "$FANOUT_STATE/pids"
sleep 300 &
echo $! >> "$FANOUT_STATE/pids"
echo begun >> "$FANOUT_STATE/begun"
wait`
	// PiLike runs its work as pi's bash tool does, detached in a session of
	// its own, out of reach of its process group, and kills it when it is
	// told to stop with SIGTERM or SIGHUP, with SIGKILL, as pi does.
	PiLike = `stopping=
trap 'stopping=yes' TERM HUP
setsid sleep 300 &
child=$!
echo $child >> "$FANOUT_STATE/pids"
echo begun >> "$FANOUT_STATE/begun"
while [ -z "$stopping" ]; do wait; done
kill -9 $child
exit 143`
	// Stubborn, and the child it starts, ignore SIGTERM.
	Stubborn = `trap '' TERM
echo $$ >> "$FANOUT_STATE/pids"
sleep 300 &
echo $! >> "$FANOUT_STATE/pids"
echo begun >> "$FANOUT_STATE/begun"
wait`
	// LeavesAChild starts a child and leaves it running when it answers.
	LeavesAChild = `sleep 300 > /dev/null 2>&1 &
echo $! >> "$FANOUT_STATE/pids"`
	// WaitForTheSlowOnes waits until two others have begun.
	WaitForTheSlowOnes = `while [ "$(cat "$FANOUT_STATE/begun" 2>/dev/null | wc -l)" -lt 2 ]; do sleep 0.01; done`
)

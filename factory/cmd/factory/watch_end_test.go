package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/lean-software-production/my-factory-reference/factory/internal/acptest"
)

// escapes are the escape sequences a terminal is written, to be left out of
// the text it shows.
var escapes = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// How a watch is ended, in TestWatchEndedEarlyLeavesNoPinnedLine.
type watchEnd struct {
	end     func(t *testing.T, tty *os.File, daemon int, state string)
	failure bool // watch exits with a failure
}

// A watch that ends early, interrupted or because the factory went away
// under it, leaves the terminal on a clean line: the status bar, all three
// rows of it, is wiped, not left for the shell's prompt to land on. So does
// one whose run ends, which still says how.
func TestWatchEndedEarlyLeavesNoPinnedLine(t *testing.T) {
	for name, how := range map[string]watchEnd{
		"interrupted": {failure: true, end: func(t *testing.T, tty *os.File, daemon int, state string) {
			if _, err := tty.Write([]byte{3}); err != nil { // ^C, as the terminal sends SIGINT
				t.Fatal(err)
			}
		}},
		"the factory killed": {failure: true, end: func(t *testing.T, tty *os.File, daemon int, state string) {
			if err := syscall.Kill(daemon, syscall.SIGKILL); err != nil {
				t.Fatal(err)
			}
		}},
		"the run ended": {end: func(t *testing.T, tty *os.File, daemon int, state string) {
			if err := os.WriteFile(filepath.Join(state, "go"), nil, 0o644); err != nil { // let the planner finish
				t.Fatal(err)
			}
		}},
	} {
		t.Run(name, func(t *testing.T) {
			// A daemon that is killed leaves its folder; this goes last,
			// once the factory has been stopped.
			var daemonDir string
			t.Cleanup(func() {
				if daemonDir != "" {
					os.RemoveAll(daemonDir)
				}
			})
			binary := buildFactory(t)
			daemonDir = daemonStateDir(t, binary)
			state, target := t.TempDir(), oneMachineTarget(t)
			planner := acptest.Script(`echo $$ > "`+state+`/planner"
while [ -d "`+state+`" ] && [ ! -f "`+state+`/go" ]; do sleep 0.01; done`, `{"complete": true}`)
			if err := os.WriteFile(filepath.Join(target, "harness"), []byte(planner), 0o755); err != nil {
				t.Fatal(err)
			}
			// However the test ends, let the planner go, and stop it and what
			// is in its process group, which a killed daemon cannot do.
			t.Cleanup(func() {
				os.WriteFile(filepath.Join(state, "go"), nil, 0o644)
				raw, _ := os.ReadFile(filepath.Join(state, "planner"))
				pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
				if err != nil || pid <= 0 {
					return
				}
				syscall.Kill(-pid, syscall.SIGKILL)
				for deadline := time.Now().Add(10 * time.Second); !acptest.ProcessGone(pid); time.Sleep(10 * time.Millisecond) {
					if time.Now().After(deadline) {
						t.Errorf("the planner, process %d, was still running 10s after it was killed", pid)
						return
					}
				}
			})
			runFactory(t, binary, target, "--run", "r", "--line", "only", "--seed", filepath.Join(target, "seed.md"), "--target", target)
			acptest.WaitForFile(t, filepath.Join(state, "planner"))
			daemon := daemonPid(t, binary)

			watch := exec.Command(binary, "watch", "r", "--target", target)
			watch.Dir = target
			watch.Env = append(os.Environ(), "NO_COLOR=", "TERM=xterm")
			tty, err := pty.Start(watch)
			if err != nil {
				t.Fatal(err)
			}
			defer tty.Close()
			var (
				mu  sync.Mutex
				out bytes.Buffer
			)
			go func() {
				buf := make([]byte, 4096)
				for {
					n, err := tty.Read(buf)
					mu.Lock()
					out.Write(buf[:n])
					mu.Unlock()
					if err != nil {
						return
					}
				}
			}()
			shown := func() string { mu.Lock(); defer mu.Unlock(); return out.String() }
			text := func() string { return escapes.ReplaceAllString(shown(), "") }
			for deadline := time.Now().Add(10 * time.Second); !strings.Contains(text(), "round 1") && time.Now().Before(deadline); {
				time.Sleep(10 * time.Millisecond)
			}
			for _, want := range []string{"r on only · " + target, "start ▸ planner", "round 1", "run 0 tokens"} {
				if !strings.Contains(text(), want) {
					t.Fatalf("watch never showed %q in its status bar:\n%q", want, text())
				}
			}

			how.end(t, tty, daemon, state)
			done := make(chan error, 1)
			go func() { done <- watch.Wait() }()
			select {
			case err := <-done:
				var exit *exec.ExitError
				if how.failure && (!errors.As(err, &exit) || exit.ExitCode() == 0) {
					t.Errorf("watch ended with %v, not a failure", err)
				}
				if !how.failure && err != nil {
					t.Errorf("watch ended with %v, not success", err)
				}
			case <-time.After(10 * time.Second):
				watch.Process.Kill()
				t.Fatalf("watch was still running 10s after it was ended:\n%q", shown())
			}
			time.Sleep(50 * time.Millisecond) // for the last of its output
			got := shown()
			// Whatever was last drawn is wiped, and nothing is drawn after
			// that.
			wiped := strings.LastIndex(got, "\x1b[2K")
			if wiped < 0 {
				t.Fatalf("the status bar was never wiped:\n%q", got)
			}
			after := escapes.ReplaceAllString(got[wiped:], "")
			for _, part := range []string{"r on only", "start ▸", "round", "tokens", "▸", "·"} {
				if strings.Contains(after, part) {
					t.Errorf("the status bar was left on the screen, with %q after the last wipe:\n%q", part, after)
				}
			}
			if finished := strings.Contains(got, "run r finished"); finished == how.failure {
				t.Errorf("the run's end: %q is shown: %v\n%q", "run r finished", finished, got)
			}
		})
	}
}

// daemonStateDir is the folder of the daemon the factory binary starts: its
// socket, its lock and its log.
func daemonStateDir(t *testing.T, binary string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(binary)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(real))
	return filepath.Join(os.TempDir(), fmt.Sprintf("factory-%d", os.Getuid()), hex.EncodeToString(sum[:6]))
}

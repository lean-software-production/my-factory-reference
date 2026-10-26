package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/lean-software-production/my-factory-reference/factory/internal/acptest"
)

// A watch that ends early, interrupted or because the factory went away
// under it, leaves the terminal on a clean line: the pinned line is wiped,
// not left for the shell's prompt to land on.
func TestWatchEndedEarlyLeavesNoPinnedLine(t *testing.T) {
	for name, end := range map[string]func(t *testing.T, tty *os.File, daemon int){
		"interrupted": func(t *testing.T, tty *os.File, daemon int) {
			if _, err := tty.Write([]byte{3}); err != nil { // ^C, as the terminal sends SIGINT
				t.Fatal(err)
			}
		},
		"the factory killed": func(t *testing.T, tty *os.File, daemon int) {
			if err := syscall.Kill(daemon, syscall.SIGKILL); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			binary := buildFactory(t)
			state, target := t.TempDir(), oneMachineTarget(t)
			planner := acptest.Script(`echo $$ > "`+state+`/planner"
while [ ! -f "`+state+`/go" ]; do sleep 0.01; done`, `{"complete": true}`)
			if err := os.WriteFile(filepath.Join(target, "harness"), []byte(planner), 0o755); err != nil {
				t.Fatal(err)
			}
			// Let the planner go however the test ends.
			t.Cleanup(func() { os.WriteFile(filepath.Join(state, "go"), nil, 0o644) })
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
			for deadline := time.Now().Add(10 * time.Second); !strings.Contains(shown(), "▶") && time.Now().Before(deadline); {
				time.Sleep(10 * time.Millisecond)
			}
			if !strings.Contains(shown(), "▶") {
				t.Fatalf("watch never showed a pinned line:\n%q", shown())
			}

			end(t, tty, daemon)
			done := make(chan error, 1)
			go func() { done <- watch.Wait() }()
			select {
			case err := <-done:
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() == 0 {
					t.Errorf("watch ended with %v, not a failure", err)
				}
			case <-time.After(10 * time.Second):
				watch.Process.Kill()
				t.Fatalf("watch was still running 10s after it was ended:\n%q", shown())
			}
			time.Sleep(50 * time.Millisecond) // for the last of its output
			got := shown()
			// Whatever was last drawn on the line is wiped, and nothing is
			// drawn after that.
			after := got[strings.LastIndex(got, "\x1b[2K"):]
			if !strings.Contains(got, "\x1b[2K") || strings.Contains(after, "▶") {
				t.Errorf("the pinned line was left on the screen:\n%q", after)
			}
		})
	}
}

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lean-software-production/my-factory-reference/factory/internal/acptest"
)

// A machine whose ACP agent is missing, or whose pi-rpc-acp has no pi to
// run, is refused when the run is started, within seconds, naming the
// machine and what is missing. pi-rpc-acp with no pi takes the prompt and
// never answers it, so a run that started would wait for ever.
func TestAMissingHarnessIsRefusedAtTheStart(t *testing.T) {
	// A pi-rpc-acp that has no pi behaves as the real one does: it takes
	// the prompt, and never answers.
	silent := acptest.Script("sleep 300")
	for name, c := range map[string]struct{ bridge, missing string }{
		"pi is not on the PATH":         {silent, "pi"},
		"pi-rpc-acp is not on the PATH": {"", "pi-rpc-acp"},
	} {
		t.Run(name, func(t *testing.T) {
			bin := t.TempDir()
			path := bin + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin"
			if found, err := lookPathIn("pi", path); err == nil {
				t.Skipf("pi is at %s, on the PATH this test leaves", found)
			}
			if c.bridge != "" {
				if err := os.WriteFile(filepath.Join(bin, "pi-rpc-acp"), []byte(c.bridge), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			binary := buildFactory(t)
			target := oneMachineTarget(t)
			config := filepath.Join(target, ".assembly-lines", "only", "machines", "planner", "machine.json")
			if err := os.WriteFile(config, []byte(`{"harness": "pi-rpc-acp", "model": "anthropic/test"}`), 0o644); err != nil {
				t.Fatal(err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			start := exec.CommandContext(ctx, binary, "--run", "r", "--line", "only", "--seed", filepath.Join(target, "seed.md"), "--target", target)
			start.Env = append(slices.DeleteFunc(os.Environ(), func(kv string) bool { return strings.HasPrefix(kv, "PATH=") }), "PATH="+path)
			out, err := start.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("starting the run took more than 10s:\n%s", out)
			}
			if err == nil || strings.Contains(string(out), "started run") {
				t.Fatalf("the run started (%v):\n%s", err, out)
			}
			for _, want := range []string{"could not run the planner", c.missing + " is not on the PATH"} {
				if !strings.Contains(string(out), want) {
					t.Errorf("the refusal does not say %q:\n%s", want, out)
				}
			}
			if status := runFactory(t, binary, "", "status"); strings.Contains(status, "run r is running") {
				t.Errorf("the run is running:\n%s", status)
			}
		})
	}
}

// lookPathIn finds the program on the PATH given.
func lookPathIn(name, path string) (string, error) {
	for _, dir := range filepath.SplitList(path) {
		if found, err := exec.LookPath(filepath.Join(dir, name)); err == nil {
			return found, nil
		}
	}
	return "", exec.ErrNotFound
}

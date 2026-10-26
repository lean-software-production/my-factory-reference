package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lean-software-production/my-factory-reference/factory/internal/acptest"
)

// A student runs the factory by absolute path or through a symlink, from
// the repository root. Relative --seed and --target must resolve from the
// caller's working directory, not from where the binary lives, nor from
// where the factory's daemon runs, and nothing is looked for beside the
// binary: the lines are in the target.
func TestRelativePathsResolveFromCallerCwd(t *testing.T) {
	binary := buildFactory(t)
	binDir := filepath.Dir(binary)

	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "s.md"), []byte("Build a game.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	agent := acptest.Harness(t, "agent", acptest.Script("", `{"complete": true}`))

	// The assembly line is a folder in the target, which is in the caller's
	// directory, not next to the binary: its graph, and a folder for each
	// machine on it, with a login of its own.
	line := filepath.Join(cwd, "t", ".assembly-lines", "test")
	graph := "digraph { start -> planner; planner -> doer [label=\"not complete\"]; planner -> finish [label=complete]; doer -> planner }"
	if err := os.MkdirAll(line, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(line, "assembly-line.dot"), []byte(graph), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"planner", "doer"} {
		dir := filepath.Join(line, "machines", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		config := `{"harness": "` + agent + `", "model": "anthropic/test"}`
		for file, text := range map[string]string{"machine.json": config, "AGENTS.md": "# " + name + "\n", "auth.json": `{"anthropic": {}}`} {
			if err := os.WriteFile(filepath.Join(dir, file), []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}

	out := runFactory(t, binary, cwd, "--run", "r", "--line", "test", "--seed", "s.md", "--target", "t")
	out += runFactory(t, binary, cwd, "watch", "r", "--target", "t")

	if info, err := os.Stat(filepath.Join(cwd, "t")); err != nil || !info.IsDir() {
		t.Errorf("the target was not created under the caller's cwd %s (%v)\n%s", cwd, err, out)
	}
	if _, err := os.Stat(filepath.Join(binDir, "t")); err == nil {
		t.Errorf("the target was created next to the binary, in %s", binDir)
	}
	if _, err := os.Stat(filepath.Join(line, "runs", "r", "run.json")); err != nil {
		t.Errorf("the run was not kept in its line's folder in the caller's target: %v\n%s", err, out)
	}
	if !strings.Contains(out, "run r finished") {
		t.Errorf("the run did not finish:\n%s", out)
	}
}

// buildFactory builds the factory into a folder of its own. Its daemon is
// stopped when the test ends.
func buildFactory(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "factory")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("building the factory: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		if out, err := exec.Command(binary, "stop").CombinedOutput(); err != nil {
			t.Errorf("stopping the factory: %v\n%s", err, out)
		}
	})
	return binary
}

// runFactory runs the factory in dir, and fails the test unless it succeeds.
func runFactory(t *testing.T, binary, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("factory %s, in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return string(out)
}

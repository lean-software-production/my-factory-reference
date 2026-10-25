package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// A student runs the factory by absolute path or through a symlink, from
// the repository root. Relative --seed and --target must resolve from the
// caller's working directory, not from where the binary lives.
func TestRelativePathsResolveFromCallerCwd(t *testing.T) {
	binDir := t.TempDir()
	binary := filepath.Join(binDir, "factory")
	build := exec.Command("go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the factory: %v\n%s", err, out)
	}

	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "s.md"), []byte("Build a game.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	agent := filepath.Join(binDir, "agent")
	script := "#!/bin/sh\necho '{\"complete\": true}'\n"
	if err := os.WriteFile(agent, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	// Each machine has a folder next to the binary, with a login of its own.
	for _, role := range []string{"planner", "doer", "validator"} {
		dir := filepath.Join(binDir, "machines", role)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		config := `{"harness": "` + agent + `", "model": "anthropic/test"}`
		for name, text := range map[string]string{"machine.json": config, "AGENTS.md": "# " + role + "\n", "auth.json": `{"anthropic": {}}`} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}

	run := exec.Command(binary, "--seed", "s.md", "--target", "t")
	run.Dir = cwd
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("running the factory from %s: %v\n%s", cwd, err, out)
	}

	if info, err := os.Stat(filepath.Join(cwd, "t")); err != nil || !info.IsDir() {
		t.Errorf("the target was not created under the caller's cwd %s (%v)\n%s", cwd, err, out)
	}
	if _, err := os.Stat(filepath.Join(binDir, "t")); err == nil {
		t.Errorf("the target was created next to the binary, in %s", binDir)
	}
}

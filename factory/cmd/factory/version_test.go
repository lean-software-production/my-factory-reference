package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// The release binary is built with its version, and says it.
func TestVersionFlag(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "factory")
	if out, err := exec.Command("go", "build", "-ldflags", "-X main.version=2026.11.03", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("building the factory: %v\n%s", err, out)
	}
	for _, flag := range []string{"--version", "-version"} {
		out, err := exec.Command(binary, flag).Output()
		if err != nil {
			t.Fatalf("factory %s: %v", flag, err)
		}
		if got, want := string(out), "factory 2026.11.03\n"; got != want {
			t.Errorf("factory %s printed %q, not %q", flag, got, want)
		}
	}
}

// Built without a version, it says it is a development build.
func TestVersionOfADevelopmentBuild(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "factory")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("building the factory: %v\n%s", err, out)
	}
	out, err := exec.Command(binary, "--version").Output()
	if err != nil || string(out) != "factory dev\n" {
		t.Errorf("printed %q, %v", out, err)
	}
}

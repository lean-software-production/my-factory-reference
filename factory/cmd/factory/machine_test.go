package main

import (
	"context"
	"slices"
	"testing"
)

// A harness runs on the machine's folder and no other configuration: not
// yours, and none found in or above the target. The doubles cannot see
// whether the real pi would find any, so this checks how it is called.
func TestHarnessIsCalledOnTheMachinesConfigurationOnly(t *testing.T) {
	m := machine{name: "doer", dir: "/factory/machines/doer", harness: "pi", model: "openai-codex/gpt-6-sol"}
	cmd := m.command(context.Background(), "/work/target", "do it")

	if cmd.Dir != "/work/target" {
		t.Errorf("it runs in %s, not in the target", cmd.Dir)
	}
	if !slices.Contains(cmd.Env, "PI_CODING_AGENT_DIR=/factory/machines/doer") {
		t.Errorf("pi is not pointed at the machine's folder: %v", cmd.Env)
	}
	want := []string{"pi", "-p", "--no-context-files", "--no-skills", "--model", "openai-codex/gpt-6-sol",
		"--append-system-prompt", "/factory/machines/doer/AGENTS.md", "do it"}
	if !slices.Equal(cmd.Args, want) {
		t.Errorf("pi is called as %q, not as %q", cmd.Args, want)
	}
}

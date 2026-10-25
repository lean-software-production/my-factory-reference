package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A finding is recorded as a subtask, and a task is not done until its
// subtasks are: the doer ticks them off and the planner honours them. Those
// are the machines' instructions, so they are in their AGENTS.md.
func TestInstructionsKeepATaskOpenUntilItsSubtasksAreDone(t *testing.T) {
	for name, tc := range map[string]struct{ machine, want string }{
		"doer ticks subtasks off":   {"doer", "Mark a subtask as done once you have put its finding right"},
		"doer: task waits":          {"doer", "not done until every one of its subtasks is"},
		"planner leaves open tasks": {"planner", "unless it has a subtask that is not done"},
	} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "assembly-lines", "careful", "machines", tc.machine, "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.Join(strings.Fields(string(raw)), " "), tc.want) { // however it is wrapped
			t.Errorf("%s: the %s's AGENTS.md lacks %q:\n%s", name, tc.machine, tc.want, raw)
		}
	}
}

// The factory holds no instructions of its own for a machine: what a machine
// is for, how it does its job and what it looks for are in its AGENTS.md.
func TestPromptsHoldNoInstructionsOfTheMachines(t *testing.T) {
	for name, prompt := range map[string]string{
		"planner":   plannerPrompt("seed", "plan", []string{"complete"}),
		"tracking":  trackingPrompt("seed", "plan", []string{"complete"}),
		"doer":      doerPrompt("seed", "plan", `["a finding"]`, nil),
		"validator": validatorPrompt("seed", "plan", "?? a.txt", []string{"satisfied"}),
		"generic":   genericPrompt("seed", "plan", []string{"ready"}),
	} {
		for _, instruction := range []string{"You are the", "lens", "subtask", "Don't commit", "implement"} {
			if strings.Contains(prompt, instruction) {
				t.Errorf("the %s prompt holds %q:\n%s", name, instruction, prompt)
			}
		}
	}
}

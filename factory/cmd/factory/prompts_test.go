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
	for name, b := range map[string]briefing{
		"first":  {seed: "seed", plan: "plan", fields: resultFields([]string{"complete"}, false, false)},
		"worker": {seed: "seed", plan: "plan", planExists: true, before: "checker", answer: `{"satisfied": false, "findings": ["a finding"]}`, fields: resultFields(nil, true, false)},
		"gate":   {seed: "seed", plan: "plan", planExists: true, changes: "?? a.txt", fields: resultFields([]string{"satisfied"}, false, true)},
	} {
		prompt := b.String()
		for _, instruction := range []string{"You are the", "lens", "subtask", "Don't commit", "implement"} {
			if strings.Contains(prompt, instruction) {
				t.Errorf("the %s prompt holds %q:\n%s", name, instruction, prompt)
			}
		}
	}
}

// Every machine is told the same things, whatever it is called: the seed,
// the plan, what the machine before it answered (findings included) and what
// is uncommitted. Only the result it is asked for differs.
func TestEveryMachineIsGivenTheSameContext(t *testing.T) {
	b := briefing{
		seed: "/s/seed.md", plan: "/t/.factory/plan.md", planExists: true,
		before: "security", answer: `{"approved": false, "findings": ["no test"]}`, changes: "?? one.txt",
		fields: resultFields([]string{"approved"}, false, true),
	}
	prompt := b.String()
	for _, want := range []string{"/s/seed.md", "/t/.factory/plan.md", "the security", `"findings": ["no test"]`, "?? one.txt", `"approved"`} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt lacks %q:\n%s", want, prompt)
		}
	}
	if first := (briefing{seed: "s", plan: "p"}).String(); strings.Contains(first, "before you") || !strings.Contains(first, "no plan yet") {
		t.Errorf("a first machine with no plan is told:\n%s", first)
	}
}

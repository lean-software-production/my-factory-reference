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
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "tetris", "tetris-005", ".assembly-lines", "careful", "machines", tc.machine, "AGENTS.md"))
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
		seed: "/s/seed.md", plan: "/t/runs/r/plan.md", runDir: "/t/runs/r", planExists: true,
		before: "security", answer: `{"approved": false, "findings": ["no test"]}`, changes: "?? one.txt",
		fields: resultFields([]string{"approved"}, false, true),
	}
	prompt := b.String()
	for _, want := range []string{"/s/seed.md", "/t/runs/r/plan.md", "The run's folder, where you keep anything you make for the run, is /t/runs/r\n", "the security", `"findings": ["no test"]`, "?? one.txt", `"approved"`} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt lacks %q:\n%s", want, prompt)
		}
	}
	if first := (briefing{seed: "s", plan: "p"}).String(); strings.Contains(first, "before you") || !strings.Contains(first, "no plan yet") {
		t.Errorf("a first machine with no plan is told:\n%s", first)
	}
}

// A plan belongs to the machines and a run resumes it: a planner given a plan
// that exists keeps its tasks. That rule is its instructions', on every line.
func TestPlannersKeepAnExistingPlansTasks(t *testing.T) {
	for _, line := range []string{"careful", "quick"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "tetris", "tetris-005", ".assembly-lines", line, "machines", "planner", "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		text := strings.Join(strings.Fields(string(raw)), " ")
		for _, want := range []string{"## When a plan already exists", "Keep its tasks as they are, in order", "Don't add, remove, reword or split them"} {
			if !strings.Contains(text, want) {
				t.Errorf("the %s line's planner AGENTS.md lacks %q", line, want)
			}
		}
	}
}

// The three big brains' reviewers do the same job on different models: their
// folders are alike but for the model each names, so each is given the same
// instructions and the same prompt.
func TestTheThreeBigBrainsAreAlikeButForTheirModels(t *testing.T) {
	machines := filepath.Join("..", "..", "..", "tetris", "tetris-005", ".assembly-lines", "three-big-brains", "machines")
	read := func(reviewer, file string) string {
		raw, err := os.ReadFile(filepath.Join(machines, reviewer, file))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	instructions := read("reviewer_1", "AGENTS.md")
	models := map[string]bool{}
	for _, reviewer := range []string{"reviewer_1", "reviewer_2", "reviewer_3"} {
		if read(reviewer, "AGENTS.md") != instructions {
			t.Errorf("%s's AGENTS.md is not reviewer_1's", reviewer)
		}
		m, err := loadMachine(reviewer, filepath.Join(machines, reviewer))
		if err != nil {
			t.Fatal(err)
		}
		models[m.model] = true
		if m.harness != "pi" {
			t.Errorf("%s runs on %s", reviewer, m.harness)
		}
	}
	if len(models) != 3 {
		t.Errorf("the reviewers run on %d models, not three: %v", len(models), models)
	}
}

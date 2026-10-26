package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var ralphLoop = filepath.Join("..", "..", "..", "lines", "ralph-loop")

// The ride's line is a planner and a doer, with no validator and no fan
// out: the planner sends work to the doer until it is complete, the doer
// sends it back, and each machine runs pi on the model it names.
func TestTheRalphLoopLineIsAPlannerAndADoer(t *testing.T) {
	line, err := loadAssemblyLine(ralphLoop)
	if err != nil {
		t.Fatal(err)
	}
	if line.planner() != "planner" {
		t.Errorf("the planner is %q", line.planner())
	}
	machines := line.machinesOn()
	slices.Sort(machines)
	if !slices.Equal(machines, []string{"doer", "planner"}) {
		t.Errorf("the machines on the line are %v", machines)
	}
	for _, name := range machines {
		if line.branches(name) != nil {
			t.Errorf("the line fans out from %s", name)
		}
		raw, err := os.ReadFile(filepath.Join(ralphLoop, "machines", name, "machine.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), `"pi-rpc-acp"`) || !strings.Contains(string(raw), `"openai-codex/`) {
			t.Errorf("the %s's machine.json is %s", name, raw)
		}
	}
	if next, err := line.next("planner", result{"complete": false}); err != nil || next != "doer" {
		t.Errorf("an incomplete plan goes to %q (%v)", next, err)
	}
	if next, err := line.next("planner", result{"complete": true}); err != nil || next != finish {
		t.Errorf("a complete plan goes to %q (%v)", next, err)
	}
	if next, err := line.next("doer", result{"task": "one"}); err != nil || next != "planner" {
		t.Errorf("the doer's work goes to %q (%v)", next, err)
	}
}

// The line's planner keeps a plan's tasks, leaves a task open while its
// subtasks are, and reports its progress for watch to show.
func TestTheRalphLoopPlannerKeepsThePlanAndReportsProgress(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(ralphLoop, "machines", "planner", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(strings.Fields(string(raw)), " ")
	for _, want := range []string{
		"## When a plan already exists", "Keep its tasks as they are, in order", "Don't add, remove, reword or split them",
		"unless it has a subtask that is not done", "a task is not done until its subtasks are",
		`{"complete": false, "progress": {"done": 5, "total": 12}}`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the planner's AGENTS.md lacks %q", want)
		}
	}
}

// Each machine on the line ends its answer with its result alone on the
// last line, so the factory reads it whatever prose comes before.
func TestTheRalphLoopMachinesEndWithTheResultAloneOnTheLastLine(t *testing.T) {
	for _, name := range []string{"planner", "doer"} {
		raw, err := os.ReadFile(filepath.Join(ralphLoop, "machines", name, "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		text := strings.Join(strings.Fields(string(raw)), " ")
		want := "End with a blank line, then the result alone on the last line: nothing before it on that line and nothing after it."
		if !strings.Contains(text, want) {
			t.Errorf("the %s's AGENTS.md lacks %q", name, want)
		}
	}
}

package main

import (
	"strings"
	"testing"
)

// A finding is recorded as a subtask, and a task is not done until its
// subtasks are: the doer ticks them off and the planner honours them.
func TestPromptsKeepATaskOpenUntilItsSubtasksAreDone(t *testing.T) {
	for name, tc := range map[string]struct{ prompt, want string }{
		"doer ticks subtasks off":   {doerPrompt("seed", "plan", `["a finding"]`), "Mark a subtask as done once you have put its finding right"},
		"doer: task waits":          {doerPrompt("seed", "plan", `["a finding"]`), "not done until every one of its subtasks is"},
		"planner leaves open tasks": {trackingPrompt("seed", "plan"), "unless it has a subtask that is not done"},
	} {
		if !strings.Contains(tc.prompt, tc.want) {
			t.Errorf("%s: the prompt lacks %q:\n%s", name, tc.want, tc.prompt)
		}
	}
}

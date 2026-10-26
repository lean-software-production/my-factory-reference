package watch

import (
	"strings"
	"testing"
)

// Watching a run shows each line with the machine that generated it, which
// machines are running, and the tokens spent: the machine's, each
// provider's and the run's, adding up as the run goes.
func TestWatchShowsLinesRunningMachinesAndTokens(t *testing.T) {
	var out strings.Builder
	w := New(&out, "careful")
	for _, ev := range []Event{
		{Kind: Run, Run: "tetris", State: Started},
		{Kind: Start, Machine: "doer"},
		{Kind: Text, Machine: "doer", Text: "first line"},
		{Kind: Result, Machine: "doer", Text: `{"task": "alpha"}`, Result: map[string]any{"task": "alpha"}},
		{Kind: Usage, Machine: "doer", Provider: "anthropic", InputTokens: 12400, OutputTokens: 3100},
		{Kind: End, Machine: "doer"},
		{Kind: Start, Machine: "reviewer_1"},
		{Kind: Start, Machine: "reviewer_2"},
		{Kind: Usage, Machine: "reviewer_2", Provider: "google", InputTokens: 4000, OutputTokens: 200},
		{Kind: End, Machine: "reviewer_2"},
		{Kind: Usage, Machine: "doer", Provider: "anthropic", InputTokens: 600, OutputTokens: 50},
		{Kind: End, Machine: "reviewer_1"},
		{Kind: Run, Run: "tetris", State: Failed, Error: "a task hit its limit"},
	} {
		if err := w.Render(ev); err != nil {
			t.Fatal(err)
		}
	}
	want := `run tetris started, on the careful line
running    doer
doer       | first line
tokens     doer 12.4k in / 3.1k out   anthropic 15.5k   run 15.5k
running    reviewer_1
running    reviewer_1, reviewer_2
tokens     reviewer_2 4.0k in / 200 out   anthropic 15.5k   google 4.2k   run 19.7k
running    reviewer_1
tokens     doer 13.0k in / 3.1k out   anthropic 16.1k   google 4.2k   run 20.4k
run tetris failed: a task hit its limit
`
	if out.String() != want {
		t.Errorf("watching showed:\n%s\nnot:\n%s", out.String(), want)
	}
}

// A message a machine was told while it worked is shown where it was told.
func TestWatchShowsWhatAMachineWasTold(t *testing.T) {
	var out strings.Builder
	w := New(&out, "careful")
	for _, ev := range []Event{
		{Kind: Start, Machine: "doer"},
		{Kind: Text, Machine: "doer", Text: "started"},
		{Kind: Told, Machine: "doer", Text: "use curses, not print"},
		{Kind: Text, Machine: "doer", Text: "finished"},
	} {
		if err := w.Render(ev); err != nil {
			t.Fatal(err)
		}
	}
	want := `running    doer
doer       | started
told       doer: use curses, not print
doer       | finished
`
	if out.String() != want {
		t.Errorf("watching showed:\n%s\nnot:\n%s", out.String(), want)
	}
}

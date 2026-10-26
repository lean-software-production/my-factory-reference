package main

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/lean-software-production/my-factory-reference/factory/internal/watch"
)

const demoRecord = "../../../demo/efficient/.assembly-lines/efficient/runs/tetris-007/record.jsonl"
const demoGraph = "../../../demo/efficient/.assembly-lines/efficient/assembly-line.dot"

const rideRecord = "../../../tetris/release-ride/.assembly-lines/ralph-loop/runs/ride/record.jsonl"
const rideGraph = "../../../tetris/release-ride/.assembly-lines/ralph-loop/assembly-line.dot"

// eventsIn reads a run's record, one event a line.
func eventsIn(t *testing.T, path string) []watch.Event {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var events []watch.Event
	lines := bufio.NewScanner(file)
	lines.Buffer(nil, 16<<20)
	for lines.Scan() {
		if strings.TrimSpace(lines.Text()) == "" {
			continue
		}
		var ev watch.Event
		if err := json.Unmarshal(lines.Bytes(), &ev); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		events = append(events, ev)
	}
	if err := lines.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func replayOf(t *testing.T, recordPath, graphPath string) replayDoc {
	t.Helper()
	return buildReplay(lineFrom(t, graphPath), eventsIn(t, recordPath), replayNames{Run: "run", Line: "line", Target: "target"})
}

// frameAt is the frame showing at a moment of run time: the last at or before it.
func frameAt(d replayDoc, seconds float64) replayFrame {
	var at replayFrame
	for _, f := range d.Frames {
		if f.At <= seconds {
			at = f
		}
	}
	return at
}

func near(got, want float64) bool { return math.Abs(got-want) <= 0.01 }

func lastFrame(t *testing.T, d replayDoc) replayFrame {
	t.Helper()
	if len(d.Frames) == 0 {
		t.Fatal("the replay has no frames")
	}
	return d.Frames[len(d.Frames)-1]
}

func TestDemoRunTimeCutsOutTheGap(t *testing.T) {
	d := replayOf(t, demoRecord, demoGraph)
	if !near(d.Total, 2201.482) {
		t.Errorf("the run took %.3fs of run time, not 2201.482s", d.Total)
	}
	if len(d.Breaks) != 1 || !near(d.Breaks[0].At, 287.528) || d.Breaks[0].Reason != "stopped" {
		t.Errorf("the breaks are %+v, not one at 287.528s, stopped", d.Breaks)
	}
	want := runEnd{State: "finished", At: d.Total, Tokens: "613.8k"}
	if d.End != want {
		t.Errorf("the run ends %+v, not %+v", d.End, want)
	}
	if last := lastFrame(t, d); !last.Finished || len(last.Working) != 0 {
		t.Errorf("the last frame is finished %v with %v working, not finished with none", last.Finished, last.Working)
	}
}

func TestDemoFrameAtNineAndAHalfMinutes(t *testing.T) {
	f := frameAt(replayOf(t, demoRecord, demoGraph), 570)
	if !slices.Equal(f.Working, []string{"doer"}) {
		t.Errorf("working are %v, not [doer]", f.Working)
	}
	if !slices.Equal(f.Done, []string{"planner"}) {
		t.Errorf("done are %v, not [planner]", f.Done)
	}
	if f.Round != 4 {
		t.Errorf("the round is %d, not 4", f.Round)
	}
	if f.Tokens != "149.4k" {
		t.Errorf("the tokens are %s, not 149.4k", f.Tokens)
	}
	for machine, model := range map[string]string{
		"doer":                "gpt-6.1-sol · low",
		"plan_reviewer_astra": "gpt-6-astra · high",
		"plan_gate":           "gpt-6.1-sol",
	} {
		if f.Models[machine] != model {
			t.Errorf("%s shows the model %q, not %q", machine, f.Models[machine], model)
		}
	}
}

func TestDemoThreeReviewersWorkAtOnce(t *testing.T) {
	working := slices.Sorted(slices.Values(frameAt(replayOf(t, demoRecord, demoGraph), 110).Working))
	want := []string{"plan_reviewer_astra", "plan_reviewer_gemini", "plan_reviewer_opus"}
	if !slices.Equal(working, want) {
		t.Errorf("working are %v, not %v", working, want)
	}
}

func TestAnErrorEndsATurnWithoutAHandOff(t *testing.T) {
	// The planner was working when the run was stopped, at 287.528s, and its
	// turn ended "the run was stopped" a moment before the run did.
	d := replayOf(t, demoRecord, demoGraph)
	if f := frameAt(d, 287.0); !slices.Contains(f.Working, "planner") {
		t.Fatalf("the planner isn't working before the stop: working %v", f.Working)
	}
	f := frameAt(d, d.Breaks[0].At)
	if slices.Contains(f.Working, "planner") || slices.Contains(f.Done, "planner") {
		t.Errorf("the planner is still shown: working %v, done %v", f.Working, f.Done)
	}
}

func TestAFailedAndResumedRideIsOneRun(t *testing.T) {
	d := replayOf(t, rideRecord, rideGraph)
	if !near(d.Total, 507.36) {
		t.Errorf("the run took %.3fs of run time, not 507.36s", d.Total)
	}
	const failed = "failed: could not read the doer's result: no line of its answer is a JSON object:"
	if len(d.Breaks) != 1 || !near(d.Breaks[0].At, 176.572) || !strings.HasPrefix(d.Breaks[0].Reason, failed) {
		t.Errorf("the breaks are %+v, not one at 176.572s starting %q", d.Breaks, failed)
	}
	if d.End.State != "finished" {
		t.Errorf("the run ends %s, not finished", d.End.State)
	}
	if last := lastFrame(t, d); !last.Finished || len(last.Working) != 0 {
		t.Errorf("the last frame is finished %v with %v working, not finished with none", last.Finished, last.Working)
	}
}

func TestRoundsCarryOnAcrossAResume(t *testing.T) {
	l := lineFrom(t, rideGraph)
	events := eventsIn(t, rideRecord)
	starts := 0
	for _, ev := range events {
		if ev.Kind == watch.Start && ev.Machine == l.planner() {
			starts++
		}
	}
	d := buildReplay(l, events, replayNames{})
	if got := lastFrame(t, d).Round; got != starts {
		t.Errorf("the last round is %d, not %d, the planner's starts", got, starts)
	}
}

func TestFramesStartAtZeroAndNeverGoBack(t *testing.T) {
	for _, run := range []struct{ record, graph string }{
		{demoRecord, demoGraph},
		{rideRecord, rideGraph},
		{"../../../tetris/release-ride-2/.assembly-lines/ralph-loop/runs/ride/record.jsonl", "../../../tetris/release-ride-2/.assembly-lines/ralph-loop/assembly-line.dot"},
		{"../../../tetris/release-ride-3/.assembly-lines/ralph-loop/runs/ride/record.jsonl", "../../../tetris/release-ride-3/.assembly-lines/ralph-loop/assembly-line.dot"},
	} {
		d := replayOf(t, run.record, run.graph)
		if len(d.Frames) == 0 || d.Frames[0].At != 0 {
			t.Errorf("%s: the frames don't start at 0", run.record)
			continue
		}
		for i, f := range d.Frames {
			if i > 0 && f.At < d.Frames[i-1].At {
				t.Errorf("%s: frame %d at %.3fs goes back from %.3fs", run.record, i, f.At, d.Frames[i-1].At)
			}
			if f.Working == nil || f.Done == nil {
				t.Errorf("%s: frame %d has working %v and done %v, one of them null", run.record, i, f.Working, f.Done)
			}
			if len(f.Counts) != len(d.Edges) {
				t.Errorf("%s: frame %d has %d counts for %d edges", run.record, i, len(f.Counts), len(d.Edges))
			}
		}
	}
}

func TestShownModel(t *testing.T) {
	for model, want := range map[string]string{
		"openai-codex/gpt-6.1-sol:high": "gpt-6.1-sol · high",
		"openai-codex/gpt-6-sol":        "gpt-6-sol",
		"gpt-5":                         "gpt-5",
	} {
		if got := shownModel(model); got != want {
			t.Errorf("shownModel(%q) is %q, not %q", model, got, want)
		}
	}
}

func nodeOf(t *testing.T, d replayDoc, name string) replayNode {
	t.Helper()
	for _, n := range d.Nodes {
		if n.Name == name {
			return n
		}
	}
	t.Fatalf("no node %s", name)
	return replayNode{}
}

func edgeOf(t *testing.T, d replayDoc, from, to string) replayEdge {
	t.Helper()
	for _, e := range d.Edges {
		if e.From == from && e.To == to {
			return e
		}
	}
	t.Fatalf("no edge %s->%s", from, to)
	return replayEdge{}
}

func TestNodesAndEdgesOfTheDemoLine(t *testing.T) {
	d := buildReplay(lineFrom(t, demoGraph), nil, replayNames{})
	want := replayNode{Name: "plan_reviewer_opus", Label: "plan reviewer opus", Kind: "machine", Rank: 3, Row: -1}
	if got := nodeOf(t, d, "plan_reviewer_opus"); got != want {
		t.Errorf("the node is %+v, not %+v", got, want)
	}
	if got := nodeOf(t, d, "start").Kind; got != "start" {
		t.Errorf("start is a %s", got)
	}
	if got := nodeOf(t, d, "finish").Kind; got != "finish" {
		t.Errorf("finish is a %s", got)
	}
	if got, want := edgeOf(t, d, "validator", "doer"), (replayEdge{"validator", "doer", "retry", true, true}); got != want {
		t.Errorf("the edge is %+v, not %+v", got, want)
	}
	if got := edgeOf(t, d, "planner", "doer"); !got.Back || !got.SendBack {
		t.Errorf("planner->doer is %+v, not back and sending back", got)
	}

	ralph := buildReplay(lineFrom(t, "../../../lines/implement-with-ralph-loop/assembly-line.dot"), nil, replayNames{})
	if got := edgeOf(t, ralph, "ralph", "planner"); !got.Back || got.SendBack {
		t.Errorf("ralph->planner is %+v, not back without sending back", got)
	}
}

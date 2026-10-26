package main

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/lean-software-production/my-factory-reference/factory/internal/watch"
)

const demoRecord = "../../../demo/implement-with-3-big-brains/.assembly-lines/implement-with-3-big-brains/runs/tetris-007/record.jsonl"
const demoGraph = "../../../demo/implement-with-3-big-brains/.assembly-lines/implement-with-3-big-brains/assembly-line.dot"

const rideRecord = "../../../tetris/release-ride/.assembly-lines/ralph-loop/runs/ride/record.jsonl"
const rideGraph = "../../../tetris/release-ride/.assembly-lines/ralph-loop/assembly-line.dot"

// realRuns are the records of finished runs in the repository, and their lines.
var realRuns = []struct{ record, graph string }{
	{demoRecord, demoGraph},
	{rideRecord, rideGraph},
	{"../../../tetris/release-ride-2/.assembly-lines/ralph-loop/runs/ride/record.jsonl", "../../../tetris/release-ride-2/.assembly-lines/ralph-loop/assembly-line.dot"},
	{"../../../tetris/release-ride-3/.assembly-lines/ralph-loop/runs/ride/record.jsonl", "../../../tetris/release-ride-3/.assembly-lines/ralph-loop/assembly-line.dot"},
}

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
	if len(d.Breaks) != 1 {
		t.Fatalf("the breaks are %+v, not one", d.Breaks)
	}
	f := frameAt(d, d.Breaks[0].At)
	if slices.Contains(f.Working, "planner") || slices.Contains(f.Done, "planner") {
		t.Errorf("the planner is still shown: working %v, done %v", f.Working, f.Done)
	}
}

func TestAMachineStartingAgainIsNoLongerDone(t *testing.T) {
	// The validator sends the work back, and the doer starts again in the
	// same round, at 1504.99s.
	d := replayOf(t, demoRecord, demoGraph)
	f := frameAt(d, 1504.99+0.5)
	if !slices.Contains(f.Working, "doer") || slices.Contains(f.Done, "doer") {
		t.Errorf("at the doer's retry, working are %v and done %v: the doer should be working and not done", f.Working, f.Done)
	}
}

func TestNoMachineIsWorkingAndDoneAtOnce(t *testing.T) {
	for _, run := range realRuns {
		for i, f := range replayOf(t, run.record, run.graph).Frames {
			for _, m := range f.Working {
				if slices.Contains(f.Done, m) {
					t.Errorf("%s: frame %d at %.3fs has %s both working and done", run.record, i, f.At, m)
				}
			}
		}
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
	for _, run := range realRuns {
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
	// The planner sending the next task out only goes round again.
	if got := edgeOf(t, d, "planner", "doer"); !got.Back || got.SendBack {
		t.Errorf("planner->doer is %+v, not back without sending back", got)
	}

	ralph := buildReplay(lineFrom(t, "../../../lines/implement-with-ralph-loop/assembly-line.dot"), nil, replayNames{})
	if got := edgeOf(t, ralph, "ralph", "planner"); !got.Back || got.SendBack {
		t.Errorf("ralph->planner is %+v, not back without sending back", got)
	}
}

const sendBacksRecord = "testdata/replay/send-backs.jsonl"

// countsOf are a frame's counts by edge, as "from->to".
func countsOf(d replayDoc, f replayFrame) map[string]int {
	counts := map[string]int{}
	for i, e := range d.Edges {
		counts[e.From+"->"+e.To] = f.Counts[i]
	}
	return counts
}

// checkCounts says where a frame's counts differ from those wanted: the
// edges named have their counts, and, when every is set, all others none.
func checkCounts(t *testing.T, d replayDoc, f replayFrame, want map[string]int, every bool) {
	t.Helper()
	counts := countsOf(d, f)
	for edge, n := range want {
		if _, ok := counts[edge]; !ok {
			t.Errorf("%s is no edge of the line, so its count of %d can't be checked", edge, n)
		}
	}
	for edge, got := range counts {
		if n, named := want[edge]; (named || every) && got != n {
			t.Errorf("%s is counted %d, not %d", edge, got, n)
		}
	}
}

func sendBacksIn(d replayDoc) []handoff {
	var sendBacks []handoff
	for _, h := range d.Handoffs {
		if h.SendBack {
			sendBacks = append(sendBacks, h)
		}
	}
	return sendBacks
}

func TestDemoEdgeCountsAtNineAndAHalfMinutes(t *testing.T) {
	d := replayOf(t, demoRecord, demoGraph)
	checkCounts(t, d, frameAt(d, 570), map[string]int{
		"start->plan_gate":                       2,
		"plan_gate->plan_author":                 1,
		"plan_author->plan_reviewer_opus":        1,
		"plan_author->plan_reviewer_gemini":      1,
		"plan_author->plan_reviewer_astra":       1,
		"plan_reviewer_opus->plan_synthesiser":   1,
		"plan_reviewer_gemini->plan_synthesiser": 1,
		"plan_reviewer_astra->plan_synthesiser":  1,
		"plan_synthesiser->doer":                 1,
		"plan_gate->planner":                     1,
		"planner->doer":                          3,
		"doer->validator":                        3,
		"validator->planner":                     3,
	}, true)
}

func TestDemoSendBacksAreTheTwoRetries(t *testing.T) {
	d := replayOf(t, demoRecord, demoGraph)
	if len(d.Handoffs) != 51 {
		t.Errorf("there are %d hand-offs, not 51", len(d.Handoffs))
	}
	if len(d.Handoffs) > 0 && (d.Handoffs[0].Machine != "plan_gate" || !near(d.Handoffs[0].At, 0.01)) {
		t.Errorf("the first hand-off is %+v, not to plan_gate at 0.01s", d.Handoffs[0])
	}
	sendBacks := sendBacksIn(d)
	if len(sendBacks) != 2 {
		t.Fatalf("the send-backs are %+v, not two", sendBacks)
	}
	for i, at := range []float64{1504.99, 1794.59} {
		if h := sendBacks[i]; h.Machine != "doer" || math.Abs(h.At-at) > 0.5 {
			t.Errorf("send-back %d is %+v, not the doer at %.2fs", i+1, h, at)
		}
	}
	// 15 doer starts in all: 12 from the planner, 1 from the synthesiser and 2 retries.
	checkCounts(t, d, lastFrame(t, d), map[string]int{
		"validator->doer":        2,
		"planner->finish":        1,
		"planner->doer":          12,
		"plan_synthesiser->doer": 1,
	}, false)
}

func TestAnUnfinishedRecordEndsUnfinished(t *testing.T) {
	raw, err := os.ReadFile(demoRecord)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitAfter(string(raw), "\n")
	partial := filepath.Join(t.TempDir(), "record.jsonl")
	if err := os.WriteFile(partial, []byte(strings.Join(lines[:201], "")), 0o644); err != nil {
		t.Fatal(err)
	}
	d := replayOf(t, partial, demoGraph)
	if d.End.State != "unfinished" || !near(d.Total, 1187.207) || d.End.Tokens != "353.3k" {
		t.Errorf("the run ends %+v after %.3fs, not unfinished after 1187.207s with 353.3k", d.End, d.Total)
	}
}

func TestSendBacksAreCountedAndMarked(t *testing.T) {
	d := replayOf(t, sendBacksRecord, demoGraph)
	last := lastFrame(t, d)
	checkCounts(t, d, last, map[string]int{
		"plan_synthesiser->plan_author":          1,
		"validator->doer":                        1,
		"validator->plan_author":                 1,
		"planner->finish":                        1,
		"plan_reviewer_opus->plan_synthesiser":   3,
		"plan_reviewer_gemini->plan_synthesiser": 3,
		"plan_reviewer_astra->plan_synthesiser":  3,
	}, false)
	var machines []string
	for _, h := range sendBacksIn(d) {
		machines = append(machines, h.Machine)
	}
	if want := []string{"plan_author", "doer", "plan_author"}; !slices.Equal(machines, want) {
		t.Errorf("the send-backs are to %v, not %v", machines, want)
	}
	if !last.Finished {
		t.Error("the last frame isn't finished")
	}
}

func TestAFailedTurnCreditsNoEdge(t *testing.T) {
	l := lineFrom(t, rideGraph)
	events := eventsIn(t, rideRecord)
	plannerStarts, doerTurns := 0, 0
	for _, ev := range events {
		switch {
		case ev.Kind == watch.Start && ev.Machine == "planner":
			plannerStarts++
		case ev.Kind == watch.End && ev.Machine == "doer" && ev.Error == "":
			doerTurns++
		}
	}
	d := buildReplay(l, events, replayNames{})
	// Every planner start but the two at the start of a segment came from the
	// doer, and the doer whose turn failed handed off nothing.
	checkCounts(t, d, lastFrame(t, d), map[string]int{
		"start->planner": 2,
		"doer->planner":  plannerStarts - 2,
	}, false)
	if doerTurns != plannerStarts-2 {
		t.Errorf("the doer ended %d turns well, and the planner started %d times", doerTurns, plannerStarts)
	}
}

func TestAFailedTurnWhileTheRunGoesOnCreditsNoEdge(t *testing.T) {
	// The ralph loop's ralph fails its turn, and the planner starts again, the
	// run going on. The failed turn handed off nothing.
	graph := "../../../lines/implement-with-ralph-loop/assembly-line.dot"
	d := replayOf(t, "testdata/replay/failed-turn.jsonl", graph)
	checkCounts(t, d, lastFrame(t, d), map[string]int{
		"start->planner":  1,
		"planner->ralph":  1,
		"ralph->planner":  0,
		"planner->finish": 1,
	}, true)
	var second *handoff
	planner := 0
	for i, h := range d.Handoffs {
		if h.Machine == "planner" {
			if planner++; planner == 2 {
				second = &d.Handoffs[i]
			}
		}
	}
	if second == nil {
		t.Fatalf("the planner doesn't start twice: hand-offs %+v", d.Handoffs)
	}
	if second.SendBack {
		t.Error("the planner's second start is a send-back")
	}
	var before, at replayFrame
	for _, f := range d.Frames {
		if f.At < second.At {
			before = f
		}
		if f.At <= second.At {
			at = f
		}
	}
	if !slices.Contains(at.Working, "planner") || slices.Contains(before.Working, "planner") {
		t.Fatalf("the frame at the planner's second start has working %v, and the frame before has %v", at.Working, before.Working)
	}
	if !slices.Equal(at.Counts, before.Counts) {
		t.Errorf("the planner's second start credited an edge: counts were %v, and are %v", before.Counts, at.Counts)
	}
}

func TestResultLineIsShortAndReadable(t *testing.T) {
	l := lineFrom(t, demoGraph)
	long := strings.Repeat("A summary that goes on. ", 13)[:300]
	for _, c := range []struct {
		machine string
		res     string
		want    string
	}{
		{"validator", `{"task":"3. Implement collision-safe horizontal movement.","retry":false,"replan":false,"satisfied":true,"findings":[],"checks":["..."]}`,
			"retry ✗ · replan ✗ · satisfied ✓ · 3. Implement collision-safe horizontal movement."},
		{"plan_gate", `{"pending":false,"ready":true}`, "pending ✗ · ready ✓"},
		{"plan_gate", `{"zeta":true,"ready":true,"alpha":false,"pending":false}`, "pending ✗ · ready ✓ · alpha ✗ · zeta ✓"},
		{"plan_reviewer_opus", `{}`, "result"},
		{"plan_synthesiser", `{"satisfied":true,"findings":[]}`, "satisfied ✓"},
		{"plan_synthesiser", `{"satisfied":true,"findings":["plain text"]}`, "satisfied ✓ · 1 finding: plain text"},
		{"plan_synthesiser", `{"satisfied":false,"findings":[3,4]}`, "satisfied ✗ · 2 findings"},
	} {
		var res map[string]any
		if err := json.Unmarshal([]byte(c.res), &res); err != nil {
			t.Fatal(err)
		}
		if got := resultLine(l, c.machine, res); got != c.want {
			t.Errorf("the result line of %s %s is %q, not %q", c.machine, c.res, got, c.want)
		}
	}

	summary := []rune(resultLine(l, "plan_reviewer_opus", map[string]any{"summary": long}))
	if len(summary) != 80 || summary[79] != '…' {
		t.Errorf("a 300-character summary is shown as %q, not cut to 80 characters ending …", string(summary))
	}

	resultAt := func(time string) map[string]any {
		for _, ev := range eventsIn(t, demoRecord) {
			if ev.Kind == watch.Result && strings.HasPrefix(ev.Time.Format("15:04:05"), time) {
				return ev.Result
			}
		}
		t.Fatalf("the demo has no result at %s", time)
		return nil
	}
	const retry = "retry ✓ · replan ✗ · satisfied ✗ · 11. Connect keyboard controls to the playable terminal session. · 1 finding: Unknown escape sequences can dispatch embedded bytes as game commands."
	if got := resultLine(l, "validator", resultAt("18:04:45")); got != retry {
		t.Errorf("the retry at 18:04:45 is shown %q, not %q", got, retry)
	}
	const two = "2 findings: Output failures during teardown are not handled reliably."
	if got := resultLine(l, "validator", resultAt("18:09:35")); !strings.HasSuffix(got, " · "+two) {
		t.Errorf("the retry at 18:09:35 is shown %q, not ending %q", got, two)
	}
}

func TestDemoLogLinesDropTheResultsJSON(t *testing.T) {
	d := replayOf(t, demoRecord, demoGraph)
	const said = "I’ll check task 3 against the accepted plan, inspect the movement changes, and run the relevant tests."
	found := false
	for i, line := range d.Lines {
		if strings.HasPrefix(line.Text, "{") {
			t.Errorf("%s says %q", line.Machine, line.Text)
		}
		if line.Machine == "validator" && line.Text == said {
			found = true
			if i+1 == len(d.Lines) || !d.Lines[i+1].Result || d.Lines[i+1].Machine != "validator" {
				t.Errorf("the validator's line isn't followed by its result")
			}
		}
	}
	if !found {
		t.Errorf("no line of the validator's says %q", said)
	}
}

func TestLogLinesStripControlCharacters(t *testing.T) {
	at := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
	events := []watch.Event{
		{Time: at, Kind: watch.Run, Run: "r", State: watch.Started},
		{Time: at, Kind: watch.Start, Machine: "plan_gate"},
		{Time: at.Add(time.Second), Kind: watch.Text, Machine: "plan_gate", Text: "red \x1b[31mtext\x1b[0m <b>bold</b>"},
		{Time: at.Add(time.Second), Kind: watch.End, Machine: "plan_gate"},
	}
	d := buildReplay(lineFrom(t, demoGraph), events, replayNames{})
	if len(d.Lines) != 1 {
		t.Fatalf("the lines are %+v, not one", d.Lines)
	}
	if text := d.Lines[0].Text; !strings.Contains(text, "<b>bold</b>") || strings.ContainsFunc(text, unicode.IsControl) {
		t.Errorf("the line is %q: it should keep <b>bold</b> and have no control characters", text)
	}
}

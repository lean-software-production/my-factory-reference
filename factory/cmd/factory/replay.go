package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/lean-software-production/my-factory-reference/factory/internal/watch"
)

// A run's replay is its record turned into what a page needs to show it
// again, in a browser, at any moment: the line drawn as a graph, and a frame
// for every change in what the line shows, which machines are working,
// which are done in this round, what has been spent. The page only draws
// it; everything it shows is worked out here, from the record and the graph
// and nothing else.
//
// A record can hold a run stopped and resumed, or failed and resumed, as
// well as one that went straight through. Its moments are given in run
// time: the time the run was going, with the gaps while it waited to be
// resumed cut out.

// replayNames are what the page calls the run in its heading.
type replayNames struct{ Run, Line, Target string }

type replayDoc struct {
	Run      string        `json:"run"`
	Line     string        `json:"line"`
	Target   string        `json:"target"`
	Nodes    []replayNode  `json:"nodes"`
	Edges    []replayEdge  `json:"edges"`
	Total    float64       `json:"total"`    // seconds of run time, idle gaps cut out
	Frames   []replayFrame `json:"frames"`   // in time order; the first has At 0
	Handoffs []handoff     `json:"handoffs"` // each machine starting, in time order
	Lines    []logLine     `json:"lines"`    // what the machines said, in time order
	Breaks   []runBreak    `json:"breaks"`
	End      runEnd        `json:"end"`
}

type replayNode struct {
	Name  string  `json:"name"`
	Label string  `json:"label"` // underscores as spaces
	Kind  string  `json:"kind"`  // "start", "machine" or "finish"
	Rank  int     `json:"rank"`
	Row   float64 `json:"row"`
}

type replayEdge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Label    string `json:"label"`
	Back     bool   `json:"back"`
	SendBack bool   `json:"sendBack"` // back, from one machine to another, neither the planner
}

type replayFrame struct {
	At       float64           `json:"at"`
	Working  []string          `json:"working"` // never null
	Done     []string          `json:"done"`    // done in this round; never null
	Round    int               `json:"round"`
	Counts   []int             `json:"counts"` // per edge, as Edges: how often the work has gone along it
	Tokens   string            `json:"tokens"` // watch.Count(input+output so far)
	Models   map[string]string `json:"models"` // machine -> shown model, from its latest usage
	Finished bool              `json:"finished"`
}

type handoff struct {
	At       float64 `json:"at"`
	Machine  string  `json:"machine"`
	SendBack bool    `json:"sendBack"`
}

type logLine struct {
	At      float64 `json:"at"`
	Machine string  `json:"machine"`
	Text    string  `json:"text"`
	Result  bool    `json:"result"`
}

type runBreak struct {
	At     float64 `json:"at"`
	Reason string  `json:"reason"`
}

type runEnd struct {
	State  string  `json:"state"` // "finished", "stopped", "failed" or "unfinished"
	At     float64 `json:"at"`
	Reason string  `json:"reason"`
	Tokens string  `json:"tokens"`
}

// unfinished is how a record ends that stops without the run ending.
const unfinished = "unfinished"

// buildReplay is the replay of a run on the line, from its record.
func buildReplay(l *assemblyLine, events []watch.Event, names replayNames) replayDoc {
	clock := timeRun(events)
	edges := edgesOf(l)
	frames, handoffs := framesOf(l, edges, events, clock.at)
	end := clock.end
	end.Tokens = frames[len(frames)-1].Tokens
	return replayDoc{
		Run: names.Run, Line: names.Line, Target: names.Target,
		Nodes:    nodesOf(l),
		Edges:    edges,
		Total:    clock.total,
		Frames:   frames,
		Handoffs: handoffs,
		Lines:    linesOf(l, events, clock.at),
		Breaks:   clock.breaks,
		End:      end,
	}
}

// nodesOf are the line's nodes where layout places them.
func nodesOf(l *assemblyLine) []replayNode {
	nodes := []replayNode{}
	for _, p := range l.layout() {
		kind := "machine"
		if p.Node == start || p.Node == finish {
			kind = p.Node
		}
		nodes = append(nodes, replayNode{Name: p.Node, Label: strings.ReplaceAll(p.Node, "_", " "), Kind: kind, Rank: p.Rank, Row: p.Row})
	}
	return nodes
}

// edgesOf are the line's edges, in the order they are written. An edge that
// goes back from one machine to another, neither of them the planner, sends
// work back: the work was not good enough and is done again. One back to
// the planner, or from it, only goes round again.
func edgesOf(l *assemblyLine) []replayEdge {
	back := l.backEdges()
	planner := l.planner()
	edges := []replayEdge{}
	for i, e := range l.edges {
		sendBack := back[i] && e.from != planner && e.to != planner
		edges = append(edges, replayEdge{From: e.from, To: e.to, Label: e.label, Back: back[i], SendBack: sendBack})
	}
	return edges
}

// runClock is the record's events placed in run time.
type runClock struct {
	at     []float64 // each event's moment, in seconds of run time
	total  float64
	breaks []runBreak // where the run ended and was later started again
	end    runEnd     // how it ended the last time, without its tokens
}

// timeRun places the record's events in run time. Each run event that
// starts the run begins a segment of it, and the next that ends the run, as
// stopped, failed or finished, ends it; a segment with no end runs to the
// last event before the next segment, or in the record. Run time is the
// time inside segments, added up.
func timeRun(events []watch.Event) runClock {
	c := runClock{at: make([]float64, len(events)), breaks: []runBreak{}}
	var (
		elapsed float64   // the run time of the segments already ended
		began   time.Time // when the segment going now began
		going   bool      // a segment is going
		last    time.Time // the time of the latest event
		ended   *runEnd   // how the latest segment to end ended
	)
	since := func(t time.Time) float64 { return max(t.Sub(began).Seconds(), 0) }
	for i, ev := range events {
		switch {
		case ev.Kind == watch.Run && ev.State == watch.Started:
			if going {
				elapsed += since(last)
				ended = &runEnd{State: unfinished, At: elapsed}
			}
			if ended != nil {
				c.breaks = append(c.breaks, runBreak{At: ended.At, Reason: ended.Reason})
			}
			going, began, ended = true, ev.Time, nil
		case ev.Ended():
			if going {
				elapsed += since(ev.Time)
				going = false
			}
			ended = &runEnd{State: ev.State, At: elapsed, Reason: endReason(ev)}
		}
		c.at[i] = elapsed
		if going {
			c.at[i] += since(ev.Time)
		}
		if i > 0 {
			c.at[i] = max(c.at[i], c.at[i-1])
		}
		last = ev.Time
	}
	if going {
		elapsed += since(last)
		ended = nil
	}
	c.total = elapsed
	c.end = runEnd{State: unfinished}
	if ended != nil {
		c.end = *ended
	}
	c.end.At = c.total
	return c
}

// endReason says why a run ended, for a run that did not finish: stopped,
// or failed and the first line of why, cut to 100 characters.
func endReason(ev watch.Event) string {
	switch ev.State {
	case watch.Stopped:
		return watch.Stopped
	case watch.Failed:
		why, _, _ := strings.Cut(ev.Error, "\n")
		if why == "" {
			return watch.Failed
		}
		return watch.Failed + ": " + cut(why, 100)
	}
	return ""
}

// cut is the text in no more than n characters, ending … if it was longer.
func cut(text string, n int) string {
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return string(runes[:n-1]) + "…"
}

// framesOf are the frames of the replay, one at 0, then one for each moment
// at which an event changes what the line shows, and its hand-offs: each
// machine starting, and whether work was sent back to it.
func framesOf(l *assemblyLine, edges []replayEdge, events []watch.Event, at []float64) ([]replayFrame, []handoff) {
	s := scene{planner: l.planner(), counts: make([]int, len(l.edges)), models: map[string]string{}}
	route := newRoute(l)
	frames := []replayFrame{s.frame(0)}
	handoffs := []handoff{}
	for i, ev := range events {
		taken := route.follow(ev)
		for _, e := range taken {
			s.counts[e]++
		}
		if ev.Kind == watch.Start {
			sentBack := slices.ContainsFunc(taken, func(e int) bool { return edges[e].SendBack })
			handoffs = append(handoffs, handoff{At: at[i], Machine: ev.Machine, SendBack: sentBack})
		}
		if !s.apply(ev) {
			continue
		}
		if f := s.frame(at[i]); frames[len(frames)-1].At == f.At {
			frames[len(frames)-1] = f
		} else {
			frames = append(frames, f)
		}
	}
	return frames, handoffs
}

// route works out, from the record, which edges the work went along: the
// record says which machines started and ended, but not along which edge.
// It is worked out with the line's own routing, from each machine's result.
//
// A machine ending its turn well hands its work off along the edges its
// result takes, or along all of them if it fans out; these edges wait for
// the machines they lead to. A machine starting takes the edges waiting for
// it, or, the first to start since the run started, the edge from start, if
// there is one. The run finishing takes the edges waiting for finish. When
// the run ends, or starts again, the edges still waiting are dropped: the
// first machine after a restart came from start.
type route struct {
	l       *assemblyLine
	waiting map[int]bool      // edges handed off along, by their place in l.edges
	results map[string]result // each working machine's result, once it has one
	begun   bool              // a machine has started since the run last started or ended
}

func newRoute(l *assemblyLine) *route {
	return &route{l: l, waiting: map[int]bool{}, results: map[string]result{}}
}

// follow takes the event into account, and gives the edges it took, by
// their place in l.edges.
func (r *route) follow(ev watch.Event) []int {
	switch ev.Kind {
	case watch.Run:
		var taken []int
		if ev.State == watch.Finished {
			taken = r.waitingFor(finish)
		}
		r.waiting, r.begun = map[int]bool{}, false
		return taken
	case watch.Start:
		delete(r.results, ev.Machine)
		first := !r.begun
		r.begun = true
		if i := r.edgeFrom(start, ev.Machine); first && i >= 0 {
			return []int{i}
		}
		return r.waitingFor(ev.Machine)
	case watch.Result:
		r.results[ev.Machine] = ev.Result
	case watch.End:
		if ev.Error == "" {
			for _, i := range r.handedOff(ev.Machine) {
				r.waiting[i] = true
			}
		}
		delete(r.results, ev.Machine)
	}
	return nil
}

// handedOff are the edges a machine that has ended its turn well hands its
// work off along: every edge out if it fans out, or else the one its result
// takes, a turn with no result taking it as empty. A result the line cannot
// route hands off along none.
func (r *route) handedOff(machine string) []int {
	if r.l.branches(machine) != nil {
		var out []int
		for i, e := range r.l.edges {
			if e.from == machine {
				out = append(out, i)
			}
		}
		return out
	}
	i, err := r.l.nextEdge(machine, r.results[machine])
	if err != nil {
		return nil
	}
	return []int{i}
}

// waitingFor takes the edges waiting for a node, and gives them in the
// order they are written.
func (r *route) waitingFor(node string) []int {
	var taken []int
	for i, e := range r.l.edges {
		if r.waiting[i] && e.to == node {
			taken = append(taken, i)
			delete(r.waiting, i)
		}
	}
	return taken
}

// edgeFrom is the place in l.edges of the edge from one node to another, or
// -1 if there is none.
func (r *route) edgeFrom(from, to string) int {
	return slices.IndexFunc(r.l.edges, func(e edge) bool { return e.from == from && e.to == to })
}

// scene is what the line shows at a moment, as the record has it so far.
type scene struct {
	planner  string
	counts   []int // how often the work has gone along each edge
	working  []string
	done     []string // done in this round
	round    int      // how many times the planner has started
	tokens   int      // spent, in and out
	models   map[string]string
	finished bool
}

// apply changes the scene as the event does, and says whether it changed
// anything shown.
//
// A machine starting is working, and not done, even if it was done earlier
// in the round: done means done this round and not working now. The planner
// starting begins a round, with none done in it. A machine ending its turn is no longer working, and
// is done in this round unless its turn failed. What it spends is counted,
// and the model it spent on shown. The run ending, however it ends, leaves
// none working.
func (s *scene) apply(ev watch.Event) bool {
	switch ev.Kind {
	case watch.Start:
		if ev.Machine == s.planner {
			s.round++
			s.done = nil
		}
		s.done = slices.DeleteFunc(s.done, func(m string) bool { return m == ev.Machine })
		if !slices.Contains(s.working, ev.Machine) {
			s.working = append(s.working, ev.Machine)
		}
	case watch.End:
		s.working = slices.DeleteFunc(s.working, func(m string) bool { return m == ev.Machine })
		if ev.Error == "" && !slices.Contains(s.done, ev.Machine) {
			s.done = append(s.done, ev.Machine)
		}
	case watch.Usage:
		s.tokens += ev.InputTokens + ev.OutputTokens
		if ev.Model != "" {
			s.models[ev.Machine] = shownModel(ev.Model)
		}
	case watch.Run:
		if !ev.Ended() {
			return false
		}
		s.working = nil
		s.finished = s.finished || ev.State == watch.Finished
	default:
		return false
	}
	return true
}

// frame is the scene as a frame at a moment of run time.
func (s *scene) frame(at float64) replayFrame {
	return replayFrame{
		At:       at,
		Working:  append([]string{}, s.working...),
		Done:     append([]string{}, s.done...),
		Round:    s.round,
		Counts:   slices.Clone(s.counts),
		Tokens:   watch.Count(s.tokens),
		Models:   maps.Clone(s.models),
		Finished: s.finished,
	}
}

// shownModel is how a model is shown beside its machine: without its
// provider, and with its effort, if it has one, after a dot.
// "openai-codex/gpt-6.1-sol:high" is shown "gpt-6.1-sol · high".
func shownModel(model string) string {
	model = model[strings.LastIndex(model, "/")+1:]
	if name, effort, ok := strings.Cut(model, ":"); ok {
		return name + " · " + effort
	}
	return model
}

// linesOf are what the machines said, in the order the record has it: each
// line of text a machine generated, and each result, shown short.
//
// A machine's result is read from the last of its lines that is JSON, and
// shown as a line of its own, so that JSON isn't shown again: a line that
// is the result's text is left out, and one that ends with it keeps only
// what comes before it.
func linesOf(l *assemblyLine, events []watch.Event, at []float64) []logLine {
	turns, results := turnsOf(events)
	lines := []logLine{}
	for i, ev := range events {
		text := ""
		switch ev.Kind {
		case watch.Text:
			text = spoken(ev.Text, results[turns[i]])
		case watch.Result:
			text = watch.WithoutControls(resultLine(l, ev.Machine, ev.Result))
		}
		if text != "" {
			lines = append(lines, logLine{At: at[i], Machine: ev.Machine, Text: text, Result: ev.Kind == watch.Result})
		}
	}
	return lines
}

// turn is one of a machine's turns: its nth start in the record, from 1.
type turn struct {
	machine string
	n       int
}

// turnsOf are the turn each event of the record is part of, and the text of
// each turn's result. An event before its machine's first start is in its
// turn 0.
func turnsOf(events []watch.Event) ([]turn, map[turn]string) {
	starts := map[string]int{}
	turns := make([]turn, len(events))
	results := map[turn]string{}
	for i, ev := range events {
		if ev.Kind == watch.Start {
			starts[ev.Machine]++
		}
		turns[i] = turn{ev.Machine, starts[ev.Machine]}
		if ev.Kind == watch.Result {
			results[turns[i]] = ev.Text
		}
	}
	return turns, results
}

// spoken is a line a machine generated as the log shows it: without its
// turn's result's text, if it is or ends with it, and without control
// characters or space around it. It is "" if nothing is left.
func spoken(text, result string) string {
	text, result = strings.TrimSpace(text), strings.TrimSpace(result)
	if result != "" {
		text = strings.TrimSuffix(text, result)
	}
	return strings.TrimSpace(watch.WithoutControls(text))
}

// resultLine is a machine's result as one short line: its true and false
// fields, those the machine's edges route on first, in the order the line
// has them, then the rest by name, each with ✓ or ✗; then its task, or else
// its summary, cut short; then, if it has findings, how many and the first.
// Other fields are left out, and a result with none of these is shown
// "result".
//
//	retry ✓ · replan ✗ · satisfied ✗ · 11. Connect keyboard controls… · 1 finding: Unknown escape sequences…
func resultLine(l *assemblyLine, machine string, res map[string]any) string {
	var parts []string
	for _, field := range truthsOf(l, machine, res) {
		mark := "✗"
		if res[field] == true {
			mark = "✓"
		}
		parts = append(parts, field+" "+mark)
	}
	if about := aboutOf(res); about != "" {
		parts = append(parts, cut(about, 80))
	}
	if findings := findingsOf(res); findings != "" {
		parts = append(parts, cut(findings, 100))
	}
	if len(parts) == 0 {
		return "result"
	}
	return strings.Join(parts, " · ")
}

// truthsOf are the fields of a result that are true or false: those the
// machine's edges route on first, in the order the line has them, then the
// rest by name.
func truthsOf(l *assemblyLine, machine string, res map[string]any) []string {
	isTruth := func(field string) bool { _, ok := res[field].(bool); return ok }
	routed := slices.DeleteFunc(l.routedFields(machine), func(field string) bool { return !isTruth(field) })
	var rest []string
	for _, field := range slices.Sorted(maps.Keys(res)) {
		if isTruth(field) && !slices.Contains(routed, field) {
			rest = append(rest, field)
		}
	}
	return append(routed, rest...)
}

// aboutOf is what a result says it is about, on one line: its task, or else
// its summary, or "" if it has neither as text.
func aboutOf(res map[string]any) string {
	for _, field := range []string{"task", "summary"} {
		if text, ok := res[field].(string); ok && strings.TrimSpace(text) != "" {
			return oneLine(text)
		}
	}
	return ""
}

// findingsOf says how many findings a result has, and the first: its issue,
// or the finding itself if it is text. It is "" if there are none.
func findingsOf(res map[string]any) string {
	findings, _ := res["findings"].([]any)
	if len(findings) == 0 {
		return ""
	}
	count := fmt.Sprintf("%d findings", len(findings))
	if len(findings) == 1 {
		count = "1 finding"
	}
	first, _ := findings[0].(string)
	if finding, ok := findings[0].(map[string]any); ok {
		first, _ = finding["issue"].(string)
	}
	if first = oneLine(first); first == "" {
		return count
	}
	return count + ": " + first
}

// oneLine is the text with every run of space, new lines among it, as one space.
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

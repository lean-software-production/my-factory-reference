package main

import (
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
	Handoffs []handoff     `json:"handoffs"` // Task 4
	Lines    []logLine     `json:"lines"`    // Task 4
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
	SendBack bool   `json:"sendBack"` // back, and to a machine other than the planner
}

type replayFrame struct {
	At       float64           `json:"at"`
	Working  []string          `json:"working"` // never null
	Done     []string          `json:"done"`    // done in this round; never null
	Round    int               `json:"round"`
	Counts   []int             `json:"counts"` // per edge, as Edges; Task 4 fills them
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
	frames := framesOf(l, events, clock.at)
	end := clock.end
	end.Tokens = frames[len(frames)-1].Tokens
	return replayDoc{
		Run: names.Run, Line: names.Line, Target: names.Target,
		Nodes:    nodesOf(l),
		Edges:    edgesOf(l),
		Total:    clock.total,
		Frames:   frames,
		Handoffs: []handoff{},
		Lines:    []logLine{},
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
// goes back to a machine other than the planner sends work back: the work
// was not good enough and is done again. One back to the planner only goes
// round again.
func edgesOf(l *assemblyLine) []replayEdge {
	back := l.backEdges()
	planner := l.planner()
	edges := []replayEdge{}
	for i, e := range l.edges {
		edges = append(edges, replayEdge{From: e.from, To: e.to, Label: e.label, Back: back[i], SendBack: back[i] && e.to != planner})
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

// framesOf are the frames of the replay: one at 0, then one for each moment
// at which an event changes what the line shows.
func framesOf(l *assemblyLine, events []watch.Event, at []float64) []replayFrame {
	s := scene{planner: l.planner(), edges: len(l.edges), models: map[string]string{}}
	frames := []replayFrame{s.frame(0)}
	for i, ev := range events {
		if !s.apply(ev) {
			continue
		}
		if f := s.frame(at[i]); frames[len(frames)-1].At == f.At {
			frames[len(frames)-1] = f
		} else {
			frames = append(frames, f)
		}
	}
	return frames
}

// scene is what the line shows at a moment, as the record has it so far.
type scene struct {
	planner  string
	edges    int
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
		Counts:   make([]int, s.edges),
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

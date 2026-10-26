package watch

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// The status bar is three rows pinned at the foot of a terminal:
//
//	ride on ralph-loop · tetris/release-ride-3
//	start ▸ planner ▸ doer ↺ planner                                round 2
//	2m 14s · planner 31.0k · doer 58.2k · run 89.2k tokens
//
// The first says the run, its line and its target. The second is the route
// the run takes along the line, with the round it is in: machines running
// now are highlighted, those that have worked in this round are green, and
// the rest are plain. The third is the clock and what has been spent.
// The bar is drawn only on a terminal.

// Route is the way a run goes along its line, as the bar shows it: the
// machines in the order the run reaches them, from start, each step one
// machine or the branches of a fan out, and the machines the line goes
// back to. The factory works it out from the line's graph; the watcher
// depends on nothing else.
type Route struct {
	Steps   [][]string // finish is never in it
	Back    []string   // the machines edges lead back to, once each
	Planner string     // the machine with an edge to finish: a round starts when it does
}

// String is the route as text: start ▸ planner ▸ doer ↺ planner.
func (r Route) String() string {
	var b strings.Builder
	b.WriteString("start")
	for _, step := range r.Steps {
		b.WriteString(" ▸ " + stepText(step))
	}
	if len(r.Back) > 0 {
		b.WriteString(" ↺ " + strings.Join(r.Back, ", "))
	}
	return b.String()
}

// stepText is one step of a route: a machine, or the branches of a fan out
// in brackets.
func stepText(step []string) string {
	if len(step) == 1 {
		return step[0]
	}
	return "[" + strings.Join(step, " ") + "]"
}

// Bar is what the status bar says of the run that the record does not.
type Bar struct {
	Run, Target string // the run's name, and the target it is in
	Route       Route
}

// SetBar tells the watcher what the status bar says. The route says which
// machine is the planner, and so what a round is.
func (wt *Watcher) SetBar(b Bar) { wt.bar = b }

// A span is some text of a row, in a style.
type span struct{ text, style string }

func spanLen(spans []span) (n int) {
	for _, s := range spans {
		n += utf8.RuneCountInString(s.text)
	}
	return n
}

// fit writes the spans as one row of at most width cells. If they do not
// fit, the row is cut and ends in an ellipsis. Every row ends by resetting
// its style.
func fit(spans []span, width int) string {
	if width <= 0 {
		return ""
	}
	cut := width > 0 && spanLen(spans) > width
	var row strings.Builder
	room := width
	if cut {
		room = width - 1
	}
	for _, s := range spans {
		text := s.text
		if n := utf8.RuneCountInString(text); n > room {
			text = string([]rune(text)[:room])
		}
		room -= utf8.RuneCountInString(text)
		if text != "" {
			row.WriteString(s.style + text)
			if s.style != "" {
				row.WriteString(reset)
			}
		}
		if room == 0 {
			break
		}
	}
	if cut {
		row.WriteString("…")
	}
	row.WriteString(reset)
	return row.String()
}

// rows are the three rows of the bar, each cut to the width of the
// terminal less one, so that none of them wraps.
func (wt *Watcher) rows() []string {
	width := 1 << 30
	if cols := wt.columns(); cols > 0 {
		width = cols - 1
	}
	return []string{
		fit(wt.runRow(), width),
		wt.routeRow(width),
		fit(wt.spendRow(), width),
	}
}

// runRow is the run, its line and its target.
func (wt *Watcher) runRow() []span {
	text := wt.bar.Run
	if text == "" {
		text = wt.run
	}
	text += " on " + wt.line
	if wt.bar.Target != "" {
		text += " · " + wt.bar.Target
	}
	return []span{{text, bold}}
}

// routeRow is the route, with the round in the last columns, as one row of
// at most width cells. The round is what changes, so it is kept: a route too
// long for the row loses its way back first, and then its end, which is
// cut and ends in an ellipsis. Only a row too narrow for the round and a
// little of the route is cut as it stands.
func (wt *Watcher) routeRow(width int) string {
	route := wt.bar.Route
	left := []span{{"start", dim}}
	for _, step := range route.Steps {
		left = append(left, span{" ▸ ", dim})
		if len(step) > 1 {
			left = append(left, span{"[", ""})
		}
		for i, m := range step {
			if i > 0 {
				left = append(left, span{" ", ""})
			}
			left = append(left, span{m, wt.mark(m)})
		}
		if len(step) > 1 {
			left = append(left, span{"]", ""})
		}
	}
	back := span{}
	if len(route.Back) > 0 {
		back = span{" ↺ " + strings.Join(route.Back, ", "), dim}
	}
	round := span{fmt.Sprintf("round %d", wt.round), dim}
	roundLen := utf8.RuneCountInString(round.text)
	const gap = 2
	row := func(left ...span) []span {
		pad := gap
		if width < 1<<30 {
			pad = max(width-spanLen(left)-roundLen, gap)
		}
		return append(left, span{strings.Repeat(" ", pad), ""}, round)
	}
	if spanLen(left)+spanLen([]span{back})+gap+roundLen <= width {
		return fit(row(append(left, back)...), width)
	}
	if width < roundLen+3 {
		return fit(row(append(left, back)...), width)
	}
	// Without the way back, if that is enough; else the route is cut so
	// that the round has its place.
	if spanLen(left)+gap+roundLen <= width {
		return fit(row(left...), width)
	}
	return fit(left, width-gap-roundLen) + strings.Repeat(" ", gap) + round.style + round.text + reset
}

// mark is how a machine is shown on the route: in its colour, reversed, if
// it is running; green if it has worked in this round; plain otherwise.
func (wt *Watcher) mark(machine string) string {
	switch {
	case slices.Contains(wt.running, machine):
		return wt.colour(machine) + inverse
	case wt.worked[machine]:
		return green
	}
	return ""
}

// spendRow is the clock, then what each machine that has spent anything
// has spent, then the run's total.
func (wt *Watcher) spendRow() []span {
	parts := []string{elapsed(wt.now.Sub(wt.start))}
	for _, m := range wt.spenders {
		t := wt.spent[m]
		parts = append(parts, m+" "+Count(t.in+t.out))
	}
	parts = append(parts, "run "+Count(wt.total)+" tokens")
	return []span{{strings.Join(parts, " · "), dim}}
}

// elapsed is a time gone, to the second: 40s, 6m 40s, 1h 5m 3s.
func elapsed(d time.Duration) string {
	s := int(max(d, 0) / time.Second)
	switch {
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm %ds", s/60, s%60)
	}
	return fmt.Sprintf("%dh %dm %ds", s/3600, s%3600/60, s%60)
}

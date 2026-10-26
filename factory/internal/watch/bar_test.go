package watch

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// screen is what a terminal shows after it is written s: its rows, with
// the styling escapes left in. It understands what the watcher writes: text,
// \n, \r, "erase the row" and "cursor up one row". The cursor is where the
// last write left it.
type screen struct {
	rows []string
	row  int
	cols int // how wide the terminal is, where text wraps; 0 for no wrapping
}

func screenOf(s string) screen { return screenOfWidth(s, 0) }

// screenOfWidth is the screen of a terminal cols wide, where text that
// reaches the last column goes on on the next row.
func screenOfWidth(s string, cols int) screen {
	sc := screen{rows: []string{""}, cols: cols}
	for i := 0; i < len(s); {
		switch {
		case strings.HasPrefix(s[i:], "\x1b[2K"):
			sc.rows[sc.row] = ""
			i += 4
		case strings.HasPrefix(s[i:], "\x1b[1A"):
			sc.row = max(sc.row-1, 0)
			i += 4
		case sgr.MatchString(s[i:]) && sgr.FindStringIndex(s[i:])[0] == 0:
			n := sgr.FindStringIndex(s[i:])[1]
			sc.rows[sc.row] += s[i : i+n]
			i += n
		case s[i] == '\r':
			i++
		case s[i] == '\n':
			sc.row++
			for len(sc.rows) <= sc.row {
				sc.rows = append(sc.rows, "")
			}
			i++
		default:
			_, n := utf8.DecodeRuneInString(s[i:])
			if sc.cols > 0 && utf8.RuneCountInString(plain(sc.rows[sc.row])) >= sc.cols {
				sc.row++
				for len(sc.rows) <= sc.row {
					sc.rows = append(sc.rows, "")
				}
			}
			sc.rows[sc.row] += s[i : i+n]
			i += n
		}
	}
	return sc
}

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// plain is the text without its styling.
func plain(s string) string { return sgr.ReplaceAllString(s, "") }

// bar is the three rows at the foot of the screen, as text, and the same
// styled.
func (sc screen) bar(t *testing.T) (plainRows, styledRows []string) {
	t.Helper()
	if sc.row != len(sc.rows)-1 || sc.row < 2 {
		t.Fatalf("the cursor is on row %d of %d: %q", sc.row, len(sc.rows), sc.rows)
	}
	styledRows = sc.rows[sc.row-2:]
	for _, r := range styledRows {
		plainRows = append(plainRows, plain(r))
	}
	return plainRows, styledRows
}

// wantBar checks that the screen shows the bar of the run: its first row.
func (sc screen) wantBar(t *testing.T) {
	t.Helper()
	rows, _ := sc.bar(t)
	if !strings.HasPrefix(rows[0], "ride on ") {
		t.Fatalf("the bar is not on the screen: %q", rows)
	}
}

// text is the screen as plain text, without the empty rows below the cursor.
func (sc screen) text() string {
	return strings.TrimRight(plain(strings.Join(sc.rows, "\n")), "\n") + "\n"
}

// above is what is on the screen above the bar.
func (sc screen) above() []string { return sc.rows[:len(sc.rows)-3] }

func at(t0 time.Time, s int, ev Event) Event {
	ev.Time = t0.Add(time.Duration(s) * time.Second)
	return ev
}

var t0 = time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)

func brains() Route {
	return Route{
		Steps:   [][]string{{"planner"}, {"doer"}, {"reviewer_1", "reviewer_2", "reviewer_3"}, {"synthesiser"}},
		Back:    []string{"doer", "planner"},
		Planner: "planner",
	}
}

func ralph() Route {
	return Route{Steps: [][]string{{"planner"}, {"doer"}}, Back: []string{"planner"}, Planner: "planner"}
}

// barWatcher is a Watcher that draws as on a terminal 120 columns wide, for
// a run on the route. (The route of three-big-brains and its round need 99.)
func barWatcher(out *bytes.Buffer, line string, route Route) *Watcher {
	wt := styled(out, line)
	wt.cols = func() int { return 120 }
	wt.SetBar(Bar{Run: "ride", Target: "tetris/release-ride-3", Route: route})
	return wt
}

// The status bar is three rows at the foot: the run, the route with the
// round, and the clock with what is spent.
func TestStatusBarHasThreeRows(t *testing.T) {
	var out bytes.Buffer
	wt := barWatcher(&out, "ralph-loop", ralph())
	render(t, wt,
		at(t0, 0, Event{Kind: Run, Run: "ride", State: Started}),
		at(t0, 1, Event{Kind: Start, Machine: "planner"}),
		at(t0, 20, Event{Kind: Usage, Machine: "planner", Provider: "openai", InputTokens: 30000, OutputTokens: 1000}),
		at(t0, 21, Event{Kind: End, Machine: "planner"}),
		at(t0, 22, Event{Kind: Start, Machine: "doer"}),
		at(t0, 134, Event{Kind: Usage, Machine: "doer", Provider: "openai", InputTokens: 50000, OutputTokens: 8200}),
	)
	rows, _ := screenOf(out.String()).bar(t)
	if want := "ride on ralph-loop · tetris/release-ride-3"; rows[0] != want {
		t.Errorf("row 1 is %q, not %q", rows[0], want)
	}
	left, round := "start ▸ planner ▸ doer ↺ planner", "round 1"
	if !strings.HasPrefix(rows[1], left) || !strings.HasSuffix(rows[1], round) {
		t.Errorf("row 2 is %q, not %q ... %q", rows[1], left, round)
	}
	if got := utf8.RuneCountInString(rows[1]); got != 119 {
		t.Errorf("row 2 is %d wide, and the round should be right-aligned to the 119th column", got)
	}
	if want := "2m 14s · planner 31.0k · doer 58.2k · run 89.2k tokens"; rows[2] != want {
		t.Errorf("row 3 is %q, not %q", rows[2], want)
	}
}

// Before anything is spent, the clock and the run's total still show.
func TestStatusBarBeforeAnythingIsSpent(t *testing.T) {
	var out bytes.Buffer
	wt := barWatcher(&out, "ralph-loop", ralph())
	render(t, wt, at(t0, 0, Event{Kind: Run, Run: "ride", State: Started}))
	rows, _ := screenOf(out.String()).bar(t)
	if rows[2] != "0s · run 0 tokens" {
		t.Errorf("row 3 is %q", rows[2])
	}
	if !strings.HasSuffix(rows[1], "round 0") {
		t.Errorf("row 2 is %q, and the planner has not started", rows[1])
	}
}

// Only the machines that have spent something are listed, in the order
// they first spent.
func TestStatusBarListsOnlyMachinesThatSpent(t *testing.T) {
	var out bytes.Buffer
	wt := barWatcher(&out, "three-big-brains", brains())
	render(t, wt,
		at(t0, 0, Event{Kind: Run, Run: "ride", State: Started}),
		at(t0, 5, Event{Kind: Usage, Machine: "reviewer_2", Provider: "g", InputTokens: 900, OutputTokens: 100}),
		at(t0, 6, Event{Kind: Usage, Machine: "planner", Provider: "o", InputTokens: 500}),
		at(t0, 7, Event{Kind: Usage, Machine: "reviewer_2", Provider: "g", InputTokens: 1000}),
	)
	rows, _ := screenOf(out.String()).bar(t)
	if want := "7s · reviewer_2 2.0k · planner 500 · run 2.5k tokens"; rows[2] != want {
		t.Errorf("row 3 is %q, not %q", rows[2], want)
	}
}

// The round is how many times the planner has started.
func TestRoundCountsThePlannersStarts(t *testing.T) {
	var out bytes.Buffer
	wt := barWatcher(&out, "ralph-loop", ralph())
	round := func() string {
		rows, _ := screenOf(out.String()).bar(t)
		_, r, _ := strings.Cut(strings.TrimSpace(rows[1]), "round ")
		return r
	}
	render(t, wt, at(t0, 0, Event{Kind: Run, Run: "ride", State: Started}))
	if got := round(); got != "0" {
		t.Errorf("before the planner starts, round %s", got)
	}
	for i := 1; i <= 3; i++ {
		render(t, wt,
			at(t0, 10*i, Event{Kind: Start, Machine: "planner"}),
			at(t0, 10*i+1, Event{Kind: End, Machine: "planner"}),
			at(t0, 10*i+2, Event{Kind: Start, Machine: "doer"}),
			at(t0, 10*i+3, Event{Kind: End, Machine: "doer"}),
		)
		if got := round(); got != fmt.Sprint(i) {
			t.Errorf("after the planner's start %d, round %s", i, got)
		}
	}
}

// A watcher that attaches late is given the record from its start, all at
// once: the round and the machines running are right at the end of it.
func TestAttachingLateGetsTheRoundAndTheRunningMachinesRight(t *testing.T) {
	var out bytes.Buffer
	wt := barWatcher(&out, "three-big-brains", brains())
	render(t, wt,
		at(t0, 0, Event{Kind: Run, Run: "ride", State: Started}),
		at(t0, 1, Event{Kind: Start, Machine: "planner"}), at(t0, 2, Event{Kind: End, Machine: "planner"}),
		at(t0, 3, Event{Kind: Start, Machine: "doer"}), at(t0, 4, Event{Kind: End, Machine: "doer"}),
		at(t0, 5, Event{Kind: Start, Machine: "reviewer_1"}), at(t0, 5, Event{Kind: Start, Machine: "reviewer_2"}),
		at(t0, 5, Event{Kind: Start, Machine: "reviewer_3"}),
		at(t0, 9, Event{Kind: End, Machine: "reviewer_3"}),
		at(t0, 10, Event{Kind: End, Machine: "reviewer_1"}), at(t0, 10, Event{Kind: End, Machine: "reviewer_2"}),
		at(t0, 11, Event{Kind: Start, Machine: "synthesiser"}), at(t0, 12, Event{Kind: End, Machine: "synthesiser"}),
		at(t0, 13, Event{Kind: Start, Machine: "planner"}), at(t0, 14, Event{Kind: End, Machine: "planner"}),
		at(t0, 15, Event{Kind: Start, Machine: "doer"}), at(t0, 16, Event{Kind: End, Machine: "doer"}),
		at(t0, 17, Event{Kind: Start, Machine: "reviewer_1"}), at(t0, 17, Event{Kind: Start, Machine: "reviewer_2"}),
		at(t0, 17, Event{Kind: Start, Machine: "reviewer_3"}), at(t0, 18, Event{Kind: End, Machine: "reviewer_2"}),
	)
	rows, styledRows := screenOf(out.String()).bar(t)
	if !strings.HasSuffix(rows[1], "round 2") {
		t.Errorf("row 2 is %q, not round 2", rows[1])
	}
	for _, m := range []string{"reviewer_1", "reviewer_3"} {
		if !strings.Contains(styledRows[1], running(wt, m)) {
			t.Errorf("%s is running and is not highlighted:\n%q", m, styledRows[1])
		}
	}
	if !strings.Contains(styledRows[1], green+"reviewer_2"+reset) {
		t.Errorf("reviewer_2 has finished and is not green:\n%q", styledRows[1])
	}
}

// running is the machine's name as it is highlighted while it runs: in its
// colour, reversed.
func running(wt *Watcher, machine string) string {
	return wt.colours[machine] + inverse + machine + reset
}

// During a fan out, the machines running are highlighted, those that have
// worked this round are green, and the rest are plain.
func TestRunningAndWorkedMachinesAreMarkedOnTheRoute(t *testing.T) {
	var out bytes.Buffer
	wt := barWatcher(&out, "three-big-brains", brains())
	render(t, wt,
		at(t0, 0, Event{Kind: Run, Run: "ride", State: Started}),
		at(t0, 1, Event{Kind: Start, Machine: "planner"}), at(t0, 2, Event{Kind: End, Machine: "planner"}),
		at(t0, 3, Event{Kind: Start, Machine: "doer"}), at(t0, 4, Event{Kind: End, Machine: "doer"}),
		at(t0, 5, Event{Kind: Start, Machine: "reviewer_1"}),
		at(t0, 5, Event{Kind: Start, Machine: "reviewer_2"}),
		at(t0, 5, Event{Kind: Start, Machine: "reviewer_3"}),
		at(t0, 8, Event{Kind: End, Machine: "reviewer_1"}),
	)
	_, rows := screenOf(out.String()).bar(t)
	row := rows[1]
	for _, m := range []string{"reviewer_2", "reviewer_3"} {
		if !strings.Contains(row, running(wt, m)) {
			t.Errorf("%s is running and is not highlighted:\n%q", m, row)
		}
	}
	for _, m := range []string{"planner", "doer", "reviewer_1"} {
		if !strings.Contains(row, green+m+reset) {
			t.Errorf("%s has worked and is not green:\n%q", m, row)
		}
	}
	// The synthesiser has not, and is plain: nothing is written around it.
	if !strings.Contains(row, " ▸ "+reset+"synthesiser"+dim) {
		t.Errorf("the synthesiser is not plain:\n%q", row)
	}
	// And the next round starts them all again.
	out.Reset()
	render(t, wt,
		at(t0, 9, Event{Kind: End, Machine: "reviewer_2"}), at(t0, 9, Event{Kind: End, Machine: "reviewer_3"}),
		at(t0, 10, Event{Kind: Start, Machine: "synthesiser"}), at(t0, 11, Event{Kind: End, Machine: "synthesiser"}),
		at(t0, 12, Event{Kind: Start, Machine: "planner"}),
	)
	_, rows = screenOf(out.String()).bar(t)
	if !strings.Contains(rows[1], running(wt, "planner")) {
		t.Errorf("the planner is running and is not highlighted:\n%q", rows[1])
	}
	for _, m := range []string{"doer", "reviewer_1", "reviewer_2", "reviewer_3", "synthesiser"} {
		if strings.Contains(rows[1], green+m+reset) || strings.Contains(rows[1], inverse+m) {
			t.Errorf("%s has not worked in the new round and is marked:\n%q", m, rows[1])
		}
	}
}

// A planner under another name still counts, because it is found from the
// graph and not by name.
func TestRoundCountsThePlannerWhateverItIsCalled(t *testing.T) {
	route := Route{Steps: [][]string{{"lead"}, {"worker"}}, Back: []string{"lead"}, Planner: "lead"}
	var out bytes.Buffer
	wt := barWatcher(&out, "ralph-loop", route)
	render(t, wt,
		at(t0, 0, Event{Kind: Run, Run: "ride", State: Started}),
		at(t0, 1, Event{Kind: Start, Machine: "lead"}), at(t0, 2, Event{Kind: End, Machine: "lead"}),
		at(t0, 3, Event{Kind: Start, Machine: "worker"}), at(t0, 4, Event{Kind: End, Machine: "worker"}),
		at(t0, 5, Event{Kind: Start, Machine: "lead"}),
	)
	rows, _ := screenOf(out.String()).bar(t)
	if !strings.HasSuffix(rows[1], "round 2") {
		t.Errorf("row 2 is %q, not round 2", rows[1])
	}
	// A machine that is merely called "planner" is not the planner.
	out.Reset()
	wt = barWatcher(&out, "ralph-loop", route)
	render(t, wt, at(t0, 0, Event{Kind: Run, Run: "ride", State: Started}), at(t0, 1, Event{Kind: Start, Machine: "planner"}))
	rows, _ = screenOf(out.String()).bar(t)
	if !strings.HasSuffix(rows[1], "round 0") {
		t.Errorf("row 2 is %q, not round 0", rows[1])
	}
}

// Status rows are cut to the terminal's width less one, ending in an
// ellipsis, so that nothing wraps. The route row keeps its round: the
// route is cut, not the round.
func TestStatusRowsAreCutToTheTerminal(t *testing.T) {
	bar := func(cols int) []string {
		var out bytes.Buffer
		wt := barWatcher(&out, "three-big-brains", brains())
		wt.cols = func() int { return cols }
		render(t, wt,
			at(t0, 0, Event{Kind: Run, Run: "ride", State: Started}),
			at(t0, 1, Event{Kind: Start, Machine: "planner"}),
			at(t0, 2, Event{Kind: Usage, Machine: "planner", Provider: "o", InputTokens: 31000}),
			at(t0, 3, Event{Kind: Usage, Machine: "reviewer_1", Provider: "o", InputTokens: 31000}),
			at(t0, 4, Event{Kind: Usage, Machine: "reviewer_2", Provider: "o", InputTokens: 31000}),
			at(t0, 5, Event{Kind: Text, Machine: "planner", Text: "a line of text"}),
		)
		rows, _ := screenOf(out.String()).bar(t)
		return rows
	}
	whole := bar(1000) // the rows as they are when nothing is cut
	if strings.Contains(strings.Join(whole, ""), "…") {
		t.Fatalf("rows were cut at 1000 columns:\n%q", whole)
	}
	round := "round 1"
	natural := []int{utf8.RuneCountInString(whole[0]), utf8.RuneCountInString(brains().String()) + 2 + len(round), utf8.RuneCountInString(whole[2])}
	for _, cols := range []int{120, 100, 99, 98, 80, 60, 40, 20, 12, 11, 10, 8, 2, 1} {
		t.Run(fmt.Sprint(cols), func(t *testing.T) {
			rows := bar(cols)
			room := cols - 1
			for i, r := range rows {
				n := utf8.RuneCountInString(r)
				if n > room {
					t.Errorf("row %d is %d wide, over %d:\n%q", i+1, n, room, r)
				}
				fits := natural[i] <= room
				switch {
				case fits && i != 1 && r != whole[i]:
					t.Errorf("row %d fits and was changed:\n%q", i+1, r)
				case !fits && room >= 1 && i != 1 && (!strings.HasSuffix(r, "…") || n != room):
					t.Errorf("row %d is not cut to %d, ending in an ellipsis:\n%q", i+1, room, r)
				}
			}
			// The route row keeps its round whenever there is room for it
			// and a little of the route.
			if room >= len(round)+3 && !strings.HasSuffix(rows[1], round) {
				t.Errorf("the round is gone from row 2 at %d columns:\n%q", cols, rows[1])
			}
			if natural[1] <= room && strings.Contains(rows[1], "…") {
				t.Errorf("row 2 fits and was cut at %d columns:\n%q", cols, rows[1])
			}
			// The way back goes first, quietly; after that the route is cut.
			if wayBack := utf8.RuneCountInString(" ↺ doer, planner"); room >= len(round)+3 && natural[1]-wayBack > room && !strings.Contains(rows[1], "…") {
				t.Errorf("the route was shortened without an ellipsis:\n%q", rows[1])
			}
			if room < len(round)+3 && room >= 1 && natural[1] > room && !strings.HasSuffix(rows[1], "…") {
				t.Errorf("row 2 is not cut, ending in an ellipsis, at %d columns:\n%q", cols, rows[1])
			}
		})
	}
	// At 80 columns the route loses its way back before it loses its end.
	if row := bar(80)[1]; !strings.HasPrefix(row, "start ▸ planner ▸ doer ▸ [reviewer_1 reviewer_2 reviewer_3] ▸ synthes…  ") || !strings.HasSuffix(row, "  round 1") {
		t.Errorf("row 2 at 80 columns:\n%q", row)
	}
	// And where only the way back does not fit, it alone is dropped.
	if row := bar(90)[1]; row != "start ▸ planner ▸ doer ▸ [reviewer_1 reviewer_2 reviewer_3] ▸ synthesiser"+strings.Repeat(" ", 89-73-7)+"round 1" {
		t.Errorf("row 2 at 90 columns:\n%q", row)
	}
}

// A log line wider than the terminal leaves no row of the bar behind: the
// bar is wiped from where it was drawn, and drawn again below the line.
func TestALogLineWiderThanTheTerminalLeavesNoLeftoverRow(t *testing.T) {
	var out bytes.Buffer
	wt := barWatcher(&out, "ralph-loop", ralph())
	wt.cols = func() int { return 40 }
	render(t, wt,
		at(t0, 0, Event{Kind: Run, Run: "ride", State: Started}),
		at(t0, 1, Event{Kind: Start, Machine: "planner"}),
		at(t0, 2, Event{Kind: Text, Machine: "planner", Text: strings.Repeat("long ", 30)}),
		at(t0, 3, Event{Kind: Text, Machine: "planner", Text: "short"}),
	)
	sc := screenOfWidth(out.String(), 40)
	rows, _ := sc.bar(t)
	if !strings.HasPrefix(rows[0], "ride on") || !strings.HasPrefix(rows[1], "start ▸") || !strings.HasSuffix(rows[1], "round 1") {
		t.Errorf("the bar: %q", rows)
	}
	for _, r := range sc.above() {
		if strings.Contains(plain(r), "ride on") || strings.Contains(plain(r), "round") {
			t.Errorf("a row of the bar was left above it: %q", sc.above())
		}
	}
	// The run starting, the machine running, the long line over five rows,
	// and the short one.
	if want := 1 + 1 + 5 + 1; len(sc.above()) != want {
		t.Errorf("%d rows above the bar, not %d: %q", len(sc.above()), want, sc.above())
	}
}

// A row that fits is not cut: the ralph-loop's bar at 80 columns, and at
// the width it just fits.
func TestStatusRowsThatFitAreNotCut(t *testing.T) {
	var out bytes.Buffer
	wt := barWatcher(&out, "ralph-loop", ralph())
	wt.cols = func() int { return 80 }
	render(t, wt, at(t0, 0, Event{Kind: Run, Run: "ride", State: Started}), at(t0, 1, Event{Kind: Start, Machine: "planner"}))
	screenOf(out.String()).wantBar(t)
	rows, _ := screenOf(out.String()).bar(t)
	if strings.Contains(strings.Join(rows, ""), "…") {
		t.Errorf("rows that fit were cut: %q", rows)
	}
	wt.cols = func() int { return utf8.RuneCountInString(rows[0]) + 1 } // the first row fits exactly
	render(t, wt, at(t0, 2, Event{Kind: Text, Machine: "planner", Text: "x"}))
	rows, _ = screenOf(out.String()).bar(t)
	if !strings.HasPrefix(rows[0], "ride on ") || strings.Contains(rows[0], "…") {
		t.Errorf("a row that just fits was cut: %q", rows[0])
	}
}

// Rows cut mid-style do not leave the style on: each ends in a reset.
func TestACutRowEndsItsStyle(t *testing.T) {
	var out bytes.Buffer
	wt := barWatcher(&out, "three-big-brains", brains())
	wt.cols = func() int { return 30 }
	render(t, wt,
		at(t0, 0, Event{Kind: Run, Run: "ride", State: Started}),
		at(t0, 1, Event{Kind: Start, Machine: "doer"}))
	sc := screenOf(out.String())
	sc.wantBar(t)
	_, rows := sc.bar(t)
	for i, r := range rows {
		if !strings.HasSuffix(r, reset) {
			t.Errorf("row %d does not end in a reset: %q", i+1, r)
		}
	}
}

// Log lines are written above the bar: the bar is wiped, three rows, before
// each and drawn again after it.
func TestLogLinesScrollAboveTheBar(t *testing.T) {
	var out bytes.Buffer
	wt := barWatcher(&out, "ralph-loop", ralph())
	render(t, wt,
		at(t0, 0, Event{Kind: Run, Run: "ride", State: Started}),
		at(t0, 1, Event{Kind: Start, Machine: "planner"}),
		at(t0, 2, Event{Kind: Text, Machine: "planner", Text: "thinking"}),
		at(t0, 3, Event{Kind: Text, Machine: "planner", Text: "still thinking"}),
	)
	sc := screenOf(out.String())
	rows, _ := sc.bar(t)
	var above []string
	for _, r := range sc.above() {
		above = append(above, plain(r))
	}
	want := []string{
		"run ride started, on the ralph-loop line",
		"running    planner",
		"planner    | thinking",
		"planner    | still thinking",
	}
	if strings.Join(above, "\n") != strings.Join(want, "\n") {
		t.Errorf("above the bar:\n%s\nnot:\n%s", strings.Join(above, "\n"), strings.Join(want, "\n"))
	}
	if !strings.HasPrefix(rows[0], "ride on") {
		t.Errorf("the bar: %q", rows)
	}
	// Each wipe is of three rows: two moves up for every wipe but the first.
	if ups, wipes := strings.Count(out.String(), "\x1b[1A"), strings.Count(out.String(), "\x1b[2K"); ups == 0 || wipes*2 != ups*3 {
		t.Errorf("%d moves up, %d wipes: not three rows wiped each time", ups, wipes)
	}
}

// The bar is wiped whole when the watcher is closed, and not drawn again.
func TestCloseWipesTheWholeBar(t *testing.T) {
	var out bytes.Buffer
	wt := barWatcher(&out, "ralph-loop", ralph())
	render(t, wt,
		at(t0, 0, Event{Kind: Run, Run: "ride", State: Started}),
		at(t0, 1, Event{Kind: Start, Machine: "planner"}),
		at(t0, 2, Event{Kind: Usage, Machine: "planner", Provider: "o", InputTokens: 100}))
	screenOf(out.String()).wantBar(t)
	if err := wt.Close(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	after := got[strings.LastIndex(got, "\x1b[2K"):]
	for _, part := range []string{"ride on", "start ▸", "▸", "round", "tokens", "·", "planner"} {
		if strings.Contains(after, part) {
			t.Errorf("after the last wipe, there is %q:\n%q", part, after)
		}
	}
	sc := screenOf(got)
	if r := sc.rows[sc.row]; r != "" {
		t.Errorf("the row the cursor is left on is not clean: %q", r)
	}
	if want := "run ride started, on the ralph-loop line\nrunning    planner\ntokens     planner 100 in / 0 out   o 100   run 100\n"; sc.text() != want {
		t.Errorf("the screen after closing:\n%q\nnot:\n%q", sc.text(), want)
	}
	// Once closed, nothing more is drawn.
	out.Reset()
	if err := wt.Close(); err != nil || out.Len() != 0 {
		t.Errorf("closing again: %v, wrote %q", err, out.String())
	}
	if err := wt.Tick(t0.Add(time.Hour)); err != nil || out.Len() != 0 {
		t.Errorf("a tick after closing: %v, wrote %q", err, out.String())
	}
	if err := wt.Render(at(t0, 50, Event{Kind: End, Machine: "planner"})); err != nil || strings.Contains(out.String(), "round") {
		t.Errorf("an event after closing drew the bar: %v, %q", err, out.String())
	}
}

// A run that ends clears the bar and says how it ended, and the clock is
// stopped.
func TestTheEndOfARunClearsTheBar(t *testing.T) {
	var out bytes.Buffer
	wt := barWatcher(&out, "ralph-loop", ralph())
	render(t, wt,
		at(t0, 0, Event{Kind: Run, Run: "ride", State: Started}),
		at(t0, 1, Event{Kind: Start, Machine: "planner"}),
		at(t0, 100, Event{Kind: End, Machine: "planner"}),
	)
	screenOf(out.String()).wantBar(t)
	render(t, wt, at(t0, 100, Event{Kind: Run, Run: "ride", State: Finished}))
	sc := screenOf(out.String())
	if want := "run ride started, on the ralph-loop line\nrunning    planner\nrun ride finished (1m 40s)\n"; sc.text() != want {
		t.Errorf("the screen at the end of the run:\n%q\nnot:\n%q", sc.text(), want)
	}
	out.Reset()
	if err := wt.Tick(t0.Add(time.Hour)); err != nil || out.Len() != 0 {
		t.Errorf("a tick after the end: %v, %q", err, out.String())
	}
}

// The clock ticks on while nothing happens, and only when a second has
// passed does it write.
func TestTheClockInTheBarTicks(t *testing.T) {
	var out bytes.Buffer
	wt := barWatcher(&out, "ralph-loop", ralph())
	render(t, wt, at(t0, 0, Event{Kind: Run, Run: "ride", State: Started}), at(t0, 0, Event{Kind: Start, Machine: "planner"}))
	written := out.Len()
	if err := wt.Tick(t0.Add(200 * time.Millisecond)); err != nil || out.Len() != written {
		t.Errorf("within the same second: %v, %q", err, out.String()[written:])
	}
	if err := wt.Tick(t0.Add(65 * time.Second)); err != nil || out.Len() == written {
		t.Fatalf("a minute on: %v, wrote nothing", err)
	}
	rows, _ := screenOf(out.String()).bar(t)
	if rows[2] != "1m 5s · run 0 tokens" {
		t.Errorf("row 3 after a minute: %q", rows[2])
	}
	if !strings.HasPrefix(rows[0], "ride on ralph-loop") || !strings.HasPrefix(rows[1], "start ▸") {
		t.Errorf("the bar after a tick: %q", rows)
	}
}

// Plain output has no escapes and no bar.
func TestPlainOutputHasNoBar(t *testing.T) {
	var out bytes.Buffer
	wt := New(&out, "ralph-loop")
	wt.SetBar(Bar{Run: "ride", Target: "tetris/x", Route: ralph()})
	render(t, wt,
		at(t0, 0, Event{Kind: Run, Run: "ride", State: Started}),
		at(t0, 1, Event{Kind: Start, Machine: "planner"}),
		at(t0, 2, Event{Kind: Text, Machine: "planner", Text: "hello"}),
		at(t0, 3, Event{Kind: Result, Machine: "planner", Text: `{}`, Result: map[string]any{"complete": false, "progress": map[string]any{"done": 1.0, "total": 4.0}}}),
		at(t0, 4, Event{Kind: Usage, Machine: "planner", Provider: "openai", InputTokens: 10, OutputTokens: 5}),
		at(t0, 5, Event{Kind: End, Machine: "planner"}),
		at(t0, 6, Event{Kind: Start, Machine: "doer"}),
		at(t0, 7, Event{Kind: End, Machine: "doer"}),
		at(t0, 8, Event{Kind: Run, Run: "ride", State: Finished}),
	)
	if err := wt.Tick(t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := wt.Close(); err != nil {
		t.Fatal(err)
	}
	want := `run ride started, on the ralph-loop line
running    planner
planner    | hello
tokens     planner 10 in / 5 out   openai 15   run 15
running    doer
run ride finished
`
	if out.String() != want {
		t.Errorf("plain output:\n%s\nnot:\n%s", out.String(), want)
	}
}

// What a machine says cannot reach the bar: control characters are stripped
// from it, as before, and the rows hold only the watcher's own text.
func TestMachineTextCannotRedrawTheBar(t *testing.T) {
	var out bytes.Buffer
	wt := barWatcher(&out, "ralph-loop", ralph())
	render(t, wt,
		at(t0, 0, Event{Kind: Run, Run: "ride", State: Started}),
		at(t0, 1, Event{Kind: Start, Machine: "planner"}),
		at(t0, 2, Event{Kind: Text, Machine: "planner", Text: "\x1b[3A\x1b[2Kgone\rwiped"}),
	)
	sc := screenOf(out.String())
	rows, _ := sc.bar(t)
	if !strings.HasPrefix(rows[0], "ride on ralph-loop") || !strings.HasPrefix(rows[1], "start ▸") {
		t.Errorf("the bar was redrawn by the text: %q", rows)
	}
	if above := sc.above(); len(above) < 3 || plain(above[2]) != "planner    | [3A[2Kgonewiped" {
		t.Errorf("the text was shown as %q", above)
	}
}

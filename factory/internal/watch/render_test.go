package watch

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

// styled is a Watcher that draws as it does on a terminal, into w.
func styled(w *bytes.Buffer, line string) *Watcher {
	wt := New(w, line)
	wt.styled = true
	return wt
}

func render(t *testing.T, wt *Watcher, events ...Event) {
	t.Helper()
	for _, ev := range events {
		if err := wt.Render(ev); err != nil {
			t.Fatal(err)
		}
	}
}

// Output that is not going to a terminal carries no escapes, so that the
// feature suite, and anyone piping to a file, sees plain lines.
func TestWatchPlainWhenNotATerminal(t *testing.T) {
	var out bytes.Buffer
	wt := New(&out, "careful") // a buffer is not a terminal
	render(t, wt,
		Event{Kind: Run, Run: "tetris", State: Started},
		Event{Kind: Start, Machine: "planner"},
		Event{Kind: Text, Machine: "planner", Text: "planning"},
		Event{Kind: Result, Machine: "planner", Text: `{}`, Result: map[string]any{"progress": map[string]any{"done": 1.0, "total": 4.0}}},
		Event{Kind: Usage, Machine: "planner", Provider: "openai", InputTokens: 10, OutputTokens: 5},
		Event{Kind: Told, Machine: "planner", Text: "hurry"},
		Event{Kind: End, Machine: "planner"},
		Event{Kind: Run, Run: "tetris", State: Finished},
	)
	if strings.Contains(out.String(), "\x1b") || strings.Contains(out.String(), "\r") {
		t.Errorf("plain output has escapes:\n%q", out.String())
	}
}

// NO_COLOR makes a terminal plain too.
func TestNoColorMakesATerminalPlain(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	_, tty, err := pty.Open()
	if err != nil {
		t.Skip("no pty here:", err)
	}
	defer tty.Close()
	if New(tty, "careful").styled {
		t.Error("NO_COLOR set, and the output is styled")
	}
	t.Setenv("NO_COLOR", "")
	if !New(tty, "careful").styled {
		t.Error("a terminal, and the output is not styled")
	}
}

// Progress that is missing, or makes no sense, is dropped or clamped.
func TestProgressBarToleratesBadValues(t *testing.T) {
	for name, value := range map[string]any{
		"nil":              nil,
		"not an object":    "6 of 12",
		"no total":         map[string]any{"done": 3.0},
		"zero total":       map[string]any{"total": 0.0},
		"negative total":   map[string]any{"done": 1.0, "total": -4.0},
		"done too big":     map[string]any{"done": 15.0, "total": 12.0},
		"done negative":    map[string]any{"done": -3.0, "total": 12.0},
		"done not number":  map[string]any{"done": "x"},
		"total not number": map[string]any{"done": 1.0, "total": "12"},
		"fractions":        map[string]any{"done": 2.5, "total": 7.9},
	} {
		t.Run(name, func(t *testing.T) {
			p, ok := parseProgress(value)
			if !ok {
				return // dropped
			}
			if p.done < 0 || p.done > p.total {
				t.Errorf("progress %+v is not clamped", p)
			}
			if n := strings.Count(p.bar(), "█") + strings.Count(p.bar(), "░"); n != barCells {
				t.Errorf("bar %q has %d cells, not %d", p.bar(), n, barCells)
			}
		})
	}

	// And a watcher given them shows no bar, and does not fall over.
	var out bytes.Buffer
	wt := styled(&out, "careful")
	render(t, wt,
		Event{Kind: Run, Run: "tetris", State: Started},
		Event{Kind: Result, Machine: "planner", Result: map[string]any{"progress": map[string]any{"total": 0.0}}},
		Event{Kind: Result, Machine: "planner", Result: map[string]any{"progress": "lots"}},
		Event{Kind: Result, Machine: "planner"},
	)
	if strings.Contains(out.String(), "task") || strings.Contains(out.String(), "█") {
		t.Errorf("a bad progress was shown:\n%q", out.String())
	}
}

// A pinned line wider than the terminal is cut, so that it cannot wrap.
func TestPinnedLineIsCutToTheTerminal(t *testing.T) {
	var out bytes.Buffer
	wt := styled(&out, "a-line-with-a-long-name")
	wt.cols = func() int { return 12 }
	render(t, wt, Event{Kind: Run, Run: "r", State: Started})
	if !strings.HasSuffix(out.String(), bold+"▶ a-line-wi"+reset) {
		t.Errorf("got %q", out.String())
	}
}

// A done greater than the total is shown as all done.
func TestProgressIsClamped(t *testing.T) {
	p, ok := parseProgress(map[string]any{"done": 15.0, "total": 12.0})
	if !ok || p.done != 12 || p.total != 12 || p.bar() != strings.Repeat("█", 16) {
		t.Errorf("got %+v %q, %v", p, p.bar(), ok)
	}
}

func TestProgressBarCounts(t *testing.T) {
	p, ok := parseProgress(map[string]any{"done": 5.0, "total": 12.0})
	if !ok {
		t.Fatal("progress not read")
	}
	if got := p.String(); got != "task 5/12" {
		t.Errorf("progress reads %q", got)
	}
	if got, want := p.bar(), strings.Repeat("█", 6)+strings.Repeat("░", 10); got != want {
		t.Errorf("bar is %q, not %q", got, want)
	}
	// Whole numbers that arrive as ints, from a result read in-process.
	if q, ok := parseProgress(map[string]any{"done": 5, "total": 12}); !ok || q != p {
		t.Errorf("ints read as %+v, %v", q, ok)
	}
}

func TestElapsedIsShownInMinutesAndSeconds(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0:                                     "0s",
		40*time.Second + 900*time.Millisecond: "40s",
		6*time.Minute + 40*time.Second:        "6m 40s",
		time.Hour + 5*time.Minute + 3*time.Second: "1h 5m 3s",
	} {
		if got := elapsed(d); got != want {
			t.Errorf("%v is shown as %q, not %q", d, got, want)
		}
	}
}

// 999,950 tokens is 1.0M, not 1000.0k.
func TestCountChangesUnitBeforeItReachesAThousand(t *testing.T) {
	for n, want := range map[int]string{
		0: "0", 950: "950", 999: "999", 1000: "1.0k", 12400: "12.4k",
		999_949: "999.9k", 999_950: "1.0M", 999_999: "1.0M", 1_000_000: "1.0M",
		3_140_000: "3.1M", 999_949_999: "999.9M", 999_950_000: "1.0G",
	} {
		if got := Count(n); got != want {
			t.Errorf("Count(%d) = %q, not %q", n, got, want)
		}
	}
}

// On a terminal the pinned line sits at the bottom: it is wiped before
// each line is written and drawn again after it.
func TestPinnedLineStaysAtTheBottom(t *testing.T) {
	var out bytes.Buffer
	wt := styled(&out, "careful")
	t0 := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
	render(t, wt,
		Event{Time: t0, Kind: Run, Run: "tetris", State: Started},
		Event{Time: t0.Add(time.Second), Kind: Result, Machine: "planner",
			Result: map[string]any{"progress": map[string]any{"done": 5.0, "total": 12.0}}},
		Event{Time: t0.Add(400 * time.Second), Kind: Text, Machine: "doer", Text: "hello"},
	)
	got := out.String()
	pin := "▶ careful  task 5/12  " + strings.Repeat("█", 6) + strings.Repeat("░", 10) + "  6m 40s"
	if !strings.HasSuffix(got, pin+reset) {
		t.Errorf("the output does not end with the pinned line %q:\n%q", pin, got)
	}
	if strings.Count(got, "hello") != 1 || strings.Index(got, "hello") > strings.LastIndex(got, "▶") {
		t.Errorf("the line was not written above the pinned line:\n%q", got)
	}
	if !strings.Contains(got, wipe) {
		t.Errorf("the pinned line was not wiped before a line was written:\n%q", got)
	}
	// Without progress, it reads the line and the clock.
	var bare bytes.Buffer
	render(t, styled(&bare, "quick"), Event{Time: t0, Kind: Run, Run: "r", State: Started})
	if !strings.Contains(bare.String(), "▶ quick  0s"+reset) {
		t.Errorf("a pinned line with no progress:\n%q", bare.String())
	}
}

// The clock keeps going while nothing happens.
func TestTickRedrawsTheClock(t *testing.T) {
	var out bytes.Buffer
	wt := styled(&out, "careful")
	t0 := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
	render(t, wt, Event{Time: t0, Kind: Run, Run: "tetris", State: Started})
	out.Reset()
	if err := wt.Tick(t0.Add(200 * time.Millisecond)); err != nil || out.Len() != 0 {
		t.Errorf("within the same second: %v, %q", err, out.String())
	}
	if err := wt.Tick(t0.Add(65 * time.Second)); err != nil || !strings.Contains(out.String(), "1m 5s") {
		t.Errorf("a minute on: %v, %q", err, out.String())
	}
	// Plain output has no clock.
	var plain bytes.Buffer
	pw := New(&plain, "careful")
	render(t, pw, Event{Time: t0, Kind: Run, Run: "tetris", State: Started})
	plain.Reset()
	if err := pw.Tick(t0.Add(time.Hour)); err != nil || plain.Len() != 0 {
		t.Errorf("plain: %v, %q", err, plain.String())
	}
	// And once the run has ended, the clock is stopped, and shown once.
	render(t, wt, Event{Time: t0.Add(70 * time.Second), Kind: Run, Run: "tetris", State: Finished})
	out.Reset()
	if err := wt.Tick(t0.Add(time.Hour)); err != nil || out.Len() != 0 {
		t.Errorf("after the end: %v, %q", err, out.String())
	}
}

// Each machine has a colour of its own, in the order they first appear,
// whatever their number: the fan-out's machines, too.
func TestMachinesHaveStableColours(t *testing.T) {
	var out bytes.Buffer
	wt := styled(&out, "three-big-brains")
	render(t, wt,
		Event{Kind: Start, Machine: "reviewer_1"},
		Event{Kind: Start, Machine: "reviewer_2"},
		Event{Kind: Text, Machine: "reviewer_2", Text: "b"},
		Event{Kind: Text, Machine: "reviewer_1", Text: "a"},
		Event{Kind: Text, Machine: "reviewer_2", Text: "c"},
	)
	c1 := colourOf(t, out.String(), "reviewer_1")
	c2 := colourOf(t, out.String(), "reviewer_2")
	if c1 == c2 || c1 == "" {
		t.Errorf("reviewers coloured %q and %q", c1, c2)
	}
	if strings.Count(out.String(), c2+"reviewer_2") < 3 {
		t.Errorf("reviewer_2 changed colour:\n%q", out.String())
	}
	// More machines than colours wrap round, and never run out.
	var many bytes.Buffer
	mw := styled(&many, "big")
	for i := 0; i < len(palette)+3; i++ {
		render(t, mw, Event{Kind: Text, Machine: strings.Repeat("m", i+1), Text: "x"})
	}
}

// colourOf is the escape the machine's name is written after.
func colourOf(t *testing.T, out, machine string) string {
	t.Helper()
	i := strings.Index(out, machine)
	j := strings.LastIndex(out[:i], "\x1b[")
	if i < 0 || j < 0 {
		t.Fatalf("no coloured %s in %q", machine, out)
	}
	return out[j:i]
}

// Names longer than the label column widen it, rather than breaking the
// lines up.
func TestLongMachineNamesWidenTheColumn(t *testing.T) {
	var out strings.Builder
	wt := New(&out, "big")
	render(t, wt,
		Event{Kind: Text, Machine: "doer", Text: "one"},
		Event{Kind: Text, Machine: "reviewer_for_security", Text: "two"},
		Event{Kind: Text, Machine: "doer", Text: "three"},
	)
	want := "doer       | one\n" +
		"reviewer_for_security | two\n" +
		"doer                  | three\n"
	if out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}

// The lines that are not a machine's are dimmed or marked, and what a
// machine was told stands out.
func TestTerminalLooksRightForTokensToldAndTheEnd(t *testing.T) {
	var out bytes.Buffer
	wt := styled(&out, "careful")
	t0 := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
	render(t, wt,
		Event{Time: t0, Kind: Run, Run: "tetris", State: Started},
		Event{Time: t0, Kind: Start, Machine: "doer"},
		Event{Time: t0, Kind: Usage, Machine: "doer", Provider: "openai", InputTokens: 12400, OutputTokens: 3100},
		Event{Time: t0, Kind: Told, Machine: "doer", Text: "use curses"},
		Event{Time: t0.Add(100 * time.Second), Kind: Run, Run: "tetris", State: Finished},
	)
	got := out.String()
	for _, want := range []string{
		dim + "tokens     doer 12.4k in / 3.1k out   openai 15.5k   run 15.5k" + reset,
		"told",
		"use curses",
		"run tetris finished",
		"(1m 40s)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%q", want, got)
		}
	}
	if strings.HasSuffix(got, reset) && strings.Contains(got[strings.LastIndex(got, "\n"):], "▶") {
		t.Errorf("the pinned line is still there after the run ended:\n%q", got)
	}
}

// Told the line's machines, the column is as wide as the longest from the
// first line.
func TestNamedMachinesFixTheColumnFromTheStart(t *testing.T) {
	var out strings.Builder
	wt := New(&out, "big")
	wt.Names("doer", "synthesiser")
	render(t, wt,
		Event{Kind: Text, Machine: "doer", Text: "one"},
		Event{Kind: Text, Machine: "synthesiser", Text: "two"},
	)
	if want := "doer        | one\nsynthesiser | two\n"; out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}

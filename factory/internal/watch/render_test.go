package watch

import (
	"bytes"
	"fmt"
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
		Event{Kind: Result, Machine: "planner", Text: `{}`, Result: map[string]any{"complete": false}},
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
}

// More machines than colours wrap round, and never run out.
func TestColoursWrapRoundForMoreMachinesThanColours(t *testing.T) {
	var out bytes.Buffer
	wt := styled(&out, "big")
	n := len(palette) + 3
	for i := 1; i <= n; i++ {
		render(t, wt, Event{Kind: Text, Machine: fmt.Sprintf("m%d", i), Text: "x"})
	}
	colour := func(i int) string { return colourOf(t, out.String(), fmt.Sprintf("m%d", i)) }
	for i := 1; i <= 3; i++ {
		if colour(i) == colour(i+1) {
			t.Errorf("m%d and m%d share a colour", i, i+1)
		}
		if got, want := colour(len(palette)+i), colour(i); got != want {
			t.Errorf("machine %d is coloured %q, not %q as machine %d is", len(palette)+i, got, want, i)
		}
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
	if after := got[strings.LastIndex(got, wipe):]; strings.Contains(after, "round") {
		t.Errorf("the bar is drawn again after the run ended:\n%q", after)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("the output does not end with a line ending:\n%q", got)
	}
}

// A watcher that has nothing pinned writes nothing on closing: one whose
// run has ended, and one that is plain.
func TestCloseWritesNothingWhenNothingIsPinned(t *testing.T) {
	var out bytes.Buffer
	wt := styled(&out, "careful")
	render(t, wt,
		Event{Kind: Run, Run: "tetris", State: Started},
		Event{Kind: Run, Run: "tetris", State: Finished})
	out.Reset()
	if err := wt.Close(); err != nil || out.Len() != 0 {
		t.Errorf("after the run ended: %v, wrote %q", err, out.String())
	}
	var plain bytes.Buffer
	pw := New(&plain, "careful")
	render(t, pw, Event{Kind: Run, Run: "tetris", State: Started})
	plain.Reset()
	if err := pw.Close(); err != nil || plain.Len() != 0 {
		t.Errorf("plain: %v, wrote %q", err, plain.String())
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

// What a machine says is shown without the control characters in it, as
// colours, cursor moves and carriage returns, so that a machine cannot
// redraw the screen or the pinned line. Tabs are kept.
func TestMachineTextIsShownWithoutControlCharacters(t *testing.T) {
	for name, wt := range map[string]func(*bytes.Buffer) *Watcher{
		"plain":    func(out *bytes.Buffer) *Watcher { return New(out, "careful") },
		"terminal": func(out *bytes.Buffer) *Watcher { return styled(out, "careful") },
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			render(t, wt(&out),
				Event{Kind: Run, Run: "tetris", State: Started},
				Event{Kind: Start, Machine: "doer"},
				Event{Kind: Text, Machine: "doer", Text: "\x1b[31mred\x1b[0m half\rway\tthere\a\x00"},
			)
			_, said, ok := strings.Cut(out.String(), " | ")
			if !ok {
				t.Fatalf("the machine's text was not shown:\n%q", out.String())
			}
			said, _, _ = strings.Cut(said, "\n")
			if said != "[31mred[0m halfway\tthere" {
				t.Errorf("the machine's text was shown as %q", said)
			}
		})
	}
}

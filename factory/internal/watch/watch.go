// Package watch shows a run as it goes: what each machine generates, which
// machines are running, and what they spend. The factory keeps a run's
// record as a list of events, and watching renders them in order, whether
// they are caught up on or followed live.
package watch

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"time"
)

// Kind is what an event in a run's record is about.
type Kind string

const (
	// Run is the run starting or ending: its State says which, and Error
	// why it failed.
	Run Kind = "run"
	// Start is a machine starting its turn, given Prompt.
	Start Kind = "start"
	// Text is a line a machine generated.
	Text Kind = "text"
	// Result is a machine's result: the last line of what it generated
	// that is JSON, as Text, and parsed, as Result.
	Result Kind = "result"
	// Usage is what a machine's turn spent, on its model's provider's bill.
	Usage Kind = "usage"
	// End is a machine ending its turn, and Error why it failed, if it did.
	End Kind = "end"
	// Told is a machine being told something while it took its turn, as
	// Text.
	Told Kind = "told"
)

// The states a run starts and ends in.
const (
	Started  = "started"
	Finished = "finished"
	Stopped  = "stopped"
	Failed   = "failed"
)

// Event is one entry in a run's record.
type Event struct {
	Time    time.Time      `json:"time"`
	Kind    Kind           `json:"kind"`
	Run     string         `json:"run,omitempty"`
	State   string         `json:"state,omitempty"`
	Machine string         `json:"machine,omitempty"`
	Text    string         `json:"text,omitempty"`
	Result  map[string]any `json:"result,omitempty"`
	Prompt  string         `json:"prompt,omitempty"`

	Provider     string `json:"provider,omitempty"`
	Model        string `json:"model,omitempty"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`

	Error string `json:"error,omitempty"`
}

// Ended says whether the event is the run ending, however it ended.
func (ev Event) Ended() bool {
	return ev.Kind == Run && ev.State != Started
}

// A Watcher renders the events of one run's record, in order. It keeps
// what it needs across events: which machines are running, which have
// worked in this round, what each has spent and for how long the run has
// gone.
//
// Written to a terminal, it styles what it shows: a colour for each
// machine, and a status bar of three rows pinned at the bottom with the
// run, its route and round, and its clock and spending. Written anywhere
// else, or with NO_COLOR set, it writes plain lines and nothing else, with
// no escapes.
type Watcher struct {
	w      io.Writer
	line   string // the assembly line the run executes
	styled bool   // writing to a terminal, to be styled
	cols   func() int

	bar     Bar    // what the status bar says that the record does not
	run     string // the run's name, as the record gives it
	round   int    // how many times the planner has started
	running []string
	worked  map[string]bool // machines that have worked in this round

	spent     map[string]*tokens // by machine
	spenders  []string           // the machines that have spent, in the order they first did
	bills     map[string]int     // tokens in and out, by provider
	providers []string           // in the order they first spend
	total     int

	colours map[string]string // each machine's colour
	width   int               // the widest a machine's name has been

	start, now time.Time // when the run started, and the latest time known
	live       bool      // the run is going, and its status bar is wanted
	pinned     bool      // the status bar is on the screen
	drawn      int       // the seconds of the clock it shows
}

type tokens struct{ in, out int }

// New is a Watcher that writes to w, for a run on the line.
func New(w io.Writer, line string) *Watcher {
	wt := &Watcher{w: w, line: line, width: labelWidth,
		worked: map[string]bool{}, spent: map[string]*tokens{}, bills: map[string]int{}, colours: map[string]string{}}
	wt.styled, wt.cols = terminalOf(w)
	return wt
}

// Names tells the watcher the machines the run may call on, so that the
// column of names is wide enough for the longest from the first line on.
func (wt *Watcher) Names(machines ...string) {
	for _, m := range machines {
		wt.width = max(wt.width, len(m))
	}
}

// labelWidth is how wide the column of machine names is, at least.
const labelWidth = 10

// Render shows what the event changes.
func (wt *Watcher) Render(ev Event) error {
	if !ev.Time.IsZero() {
		wt.now = ev.Time
		if wt.start.IsZero() {
			wt.start = ev.Time
		}
	}
	if ev.Machine != "" {
		wt.width = max(wt.width, len(ev.Machine))
	}
	switch ev.Kind {
	case Run:
		switch ev.State {
		case Started:
			// Whatever was running before, it is not running now.
			wt.running = nil
			wt.live = true
			wt.run = ev.Run
			if !ev.Time.IsZero() {
				wt.start = ev.Time
			}
			text := fmt.Sprintf("run %s started, on the %s line", ev.Run, wt.line)
			return wt.show(text, bold+text+reset)
		case Failed:
			return wt.ended(ev, fmt.Sprintf("run %s failed: %s", ev.Run, ev.Error), red)
		case Finished:
			return wt.ended(ev, fmt.Sprintf("run %s %s", ev.Run, ev.State), green)
		default:
			return wt.ended(ev, fmt.Sprintf("run %s %s", ev.Run, ev.State), yellow)
		}
	case Start:
		wt.live = true
		// A round is the planner starting; the machines that worked in the
		// last have not worked in this one.
		if p := wt.bar.Route.Planner; p != "" && ev.Machine == p {
			wt.round++
			clear(wt.worked)
		}
		if !slices.Contains(wt.running, ev.Machine) {
			wt.running = append(wt.running, ev.Machine)
		}
		return wt.showRunning()
	case End:
		wt.running = slices.DeleteFunc(wt.running, func(m string) bool { return m == ev.Machine })
		wt.worked[ev.Machine] = true
		if len(wt.running) == 0 {
			// Nothing to say, but the route shows the machine has finished.
			return wt.redraw()
		}
		return wt.showRunning()
	case Text:
		text := withoutControls(ev.Text)
		return wt.show(
			fmt.Sprintf("%-*s | %s", wt.width, ev.Machine, text),
			wt.label(ev.Machine)+" | "+text)
	case Result:
		// The result is already shown, as the line of text it is.
		return nil
	case Usage:
		return wt.spend(ev)
	case Told:
		return wt.show(
			fmt.Sprintf("%-*s %s: %s", wt.width, "told", ev.Machine, ev.Text),
			fmt.Sprintf("%s%-*s%s %s: %s", bold, wt.width, "told", reset, wt.name(ev.Machine), ev.Text))
	}
	return nil
}

// Tick tells the watcher the time now, so that the clock on the status bar
// goes on while nothing happens. It writes nothing unless a second has
// passed, and nothing at all when the output is plain.
func (wt *Watcher) Tick(now time.Time) error {
	if !wt.styled || !wt.live || wt.start.IsZero() {
		return nil
	}
	wt.now = now
	if wt.seconds() == wt.drawn && wt.pinned {
		return nil
	}
	return wt.redraw()
}

// redraw draws the status bar again, if there is one to draw: on a
// terminal, while the run is going.
func (wt *Watcher) redraw() error {
	if !wt.styled || !wt.live {
		return nil
	}
	return wt.write(wt.unpin() + wt.pin())
}

// ended shows the run ending, and, on a terminal, how long it took, in
// place of the status bar.
func (wt *Watcher) ended(ev Event, text, colour string) error {
	wt.live = false
	return wt.show(text, colour+text+reset+dim+" ("+elapsed(wt.now.Sub(wt.start))+")"+reset)
}

func (wt *Watcher) showRunning() error {
	names := make([]string, len(wt.running))
	for i, m := range wt.running {
		names[i] = wt.name(m)
	}
	return wt.show(
		fmt.Sprintf("%-*s %s", wt.width, "running", strings.Join(wt.running, ", ")),
		fmt.Sprintf("%s%-*s%s %s", dim, wt.width, "running", reset, strings.Join(names, ", ")))
}

// spend adds what a turn spent to its machine's, its provider's and the
// run's totals, and shows them.
func (wt *Watcher) spend(ev Event) error {
	t := wt.spent[ev.Machine]
	if t == nil {
		t = &tokens{}
		wt.spent[ev.Machine] = t
		wt.spenders = append(wt.spenders, ev.Machine)
	}
	t.in += ev.InputTokens
	t.out += ev.OutputTokens
	if _, ok := wt.bills[ev.Provider]; !ok {
		wt.providers = append(wt.providers, ev.Provider)
	}
	wt.bills[ev.Provider] += ev.InputTokens + ev.OutputTokens
	wt.total += ev.InputTokens + ev.OutputTokens

	var line strings.Builder
	fmt.Fprintf(&line, "%-*s %s %s in / %s out", wt.width, "tokens", ev.Machine, Count(t.in), Count(t.out))
	for _, provider := range wt.providers {
		fmt.Fprintf(&line, "   %s %s", provider, Count(wt.bills[provider]))
	}
	fmt.Fprintf(&line, "   run %s", Count(wt.total))
	return wt.show(line.String(), dim+line.String()+reset)
}

// Count is a number of tokens, short: 950, 12.4k, 3.1M. It moves up to the
// next unit once it would round to a thousand of the one it is in.
func Count(n int) string {
	if n < 1000 {
		return fmt.Sprint(n)
	}
	unit := 1000.0
	for _, suffix := range []string{"k", "M", "G"} {
		if v := float64(n) / unit; v < 999.95 || suffix == "G" {
			return fmt.Sprintf("%.1f%s", v, suffix)
		}
		unit *= 1000
	}
	return fmt.Sprint(n)
}

// withoutControls is the text without its C0 control characters, ESC
// among them, but for tabs: what a machine says cannot move the cursor,
// colour the screen or wipe the pinned line.
func withoutControls(text string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\t' {
			return -1
		}
		return r
	}, text)
}

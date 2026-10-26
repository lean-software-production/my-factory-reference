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

// A Watcher renders the events of one run's record, in order, as plain
// lines. It keeps what it needs across events: which machines are running
// and what each has spent.
type Watcher struct {
	w    io.Writer
	line string // the assembly line the run executes

	running   []string
	spent     map[string]*tokens // by machine
	bills     map[string]int     // tokens in and out, by provider
	providers []string           // in the order they first spend
	total     int
}

type tokens struct{ in, out int }

// New is a Watcher that writes to w, for a run on the line.
func New(w io.Writer, line string) *Watcher {
	return &Watcher{w: w, line: line, spent: map[string]*tokens{}, bills: map[string]int{}}
}

// labelWidth is how wide the column of machine names is.
const labelWidth = 10

// Render shows what the event changes.
func (wt *Watcher) Render(ev Event) error {
	switch ev.Kind {
	case Run:
		switch ev.State {
		case Started:
			// Whatever was running before, it is not running now.
			wt.running = nil
			return wt.printf("run %s started, on the %s line\n", ev.Run, wt.line)
		case Failed:
			return wt.printf("run %s failed: %s\n", ev.Run, ev.Error)
		default:
			return wt.printf("run %s %s\n", ev.Run, ev.State)
		}
	case Start:
		if !slices.Contains(wt.running, ev.Machine) {
			wt.running = append(wt.running, ev.Machine)
		}
		return wt.showRunning()
	case End:
		wt.running = slices.DeleteFunc(wt.running, func(m string) bool { return m == ev.Machine })
		if len(wt.running) == 0 {
			return nil
		}
		return wt.showRunning()
	case Text:
		return wt.printf("%-*s | %s\n", labelWidth, ev.Machine, ev.Text)
	case Usage:
		return wt.spend(ev)
	}
	// A result is already shown, as the line of text it is.
	return nil
}

func (wt *Watcher) showRunning() error {
	return wt.printf("%-*s %s\n", labelWidth, "running", strings.Join(wt.running, ", "))
}

// spend adds what a turn spent to its machine's, its provider's and the
// run's totals, and shows them.
func (wt *Watcher) spend(ev Event) error {
	t := wt.spent[ev.Machine]
	if t == nil {
		t = &tokens{}
		wt.spent[ev.Machine] = t
	}
	t.in += ev.InputTokens
	t.out += ev.OutputTokens
	if _, ok := wt.bills[ev.Provider]; !ok {
		wt.providers = append(wt.providers, ev.Provider)
	}
	wt.bills[ev.Provider] += ev.InputTokens + ev.OutputTokens
	wt.total += ev.InputTokens + ev.OutputTokens

	var line strings.Builder
	fmt.Fprintf(&line, "%-*s %s %s in / %s out", labelWidth, "tokens", ev.Machine, Count(t.in), Count(t.out))
	for _, provider := range wt.providers {
		fmt.Fprintf(&line, "   %s %s", provider, Count(wt.bills[provider]))
	}
	fmt.Fprintf(&line, "   run %s\n", Count(wt.total))
	return wt.printf("%s", line.String())
}

func (wt *Watcher) printf(format string, args ...any) error {
	_, err := fmt.Fprintf(wt.w, format, args...)
	return err
}

// Count is a number of tokens, short: 950, 12.4k, 3.1M.
func Count(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprint(n)
	case n < 1_000_000:
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	default:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
}

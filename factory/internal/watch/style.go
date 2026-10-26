package watch

import (
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

// The escapes a terminal is written. Plain output has none of them.
const (
	reset   = "\x1b[0m"
	bold    = "\x1b[1m"
	dim     = "\x1b[2m"
	red     = "\x1b[31m"
	green   = "\x1b[32m"
	yellow  = "\x1b[33m"
	inverse = "\x1b[7m"
	wipe    = "\r\x1b[2K" // wipe the line the cursor is on
	up      = "\x1b[1A"   // move the cursor up a line
)

// palette is the colours machines are given, in the order they first
// appear. A run with more machines than colours starts over.
var palette = []string{"\x1b[36m", "\x1b[33m", "\x1b[32m", "\x1b[35m", "\x1b[94m", "\x1b[91m"}

// terminalOf says whether to style what is written to w: it is a
// terminal, and NO_COLOR (see no-color.org) is not set. If so, it also
// gives how wide the terminal is, or 0 if that is not known.
func terminalOf(w any) (bool, func() int) {
	f, ok := w.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false, nil
	}
	return true, func() int {
		cols, _, err := term.GetSize(int(f.Fd()))
		if err != nil {
			return 0
		}
		return cols
	}
}

// colour is the machine's colour, given it the first time it is asked for.
func (wt *Watcher) colour(machine string) string {
	colour, ok := wt.colours[machine]
	if !ok {
		colour = palette[len(wt.colours)%len(palette)]
		wt.colours[machine] = colour
	}
	return colour
}

// name is the machine's name in its colour.
func (wt *Watcher) name(machine string) string {
	return wt.colour(machine) + machine + reset
}

// label is the machine's name in its colour, padded to the column.
func (wt *Watcher) label(machine string) string {
	return wt.name(machine) + strings.Repeat(" ", max(wt.width-len(machine), 0))
}

// show writes a line: the plain one, or, on a terminal, the styled one,
// above the status bar.
func (wt *Watcher) show(plain, styled string) error {
	if !wt.styled {
		return wt.write(plain + "\n")
	}
	return wt.write(wt.unpin() + styled + "\n" + wt.pin())
}

func (wt *Watcher) write(s string) error {
	if s == "" {
		return nil
	}
	_, err := fmt.Fprint(wt.w, s)
	return err
}

// barRows is how many rows the status bar is.
const barRows = 3

// unpin is what wipes the status bar from the screen, if it is there: the
// row the cursor is on, which is the last, and each of the rows above it,
// leaving the cursor on the first.
func (wt *Watcher) unpin() string {
	if !wt.pinned {
		return ""
	}
	wt.pinned = false
	return wipe + strings.Repeat(up+wipe, barRows-1)
}

// pin is the status bar, written at the foot of the screen, with no line
// ending after the last row, so that the next line replaces it from the
// first. It is empty once the run has ended.
func (wt *Watcher) pin() string {
	if !wt.live {
		return ""
	}
	wt.pinned, wt.drawn = true, wt.seconds()
	return strings.Join(wt.rows(), "\n")
}

// seconds is how long the run has gone, in whole seconds.
func (wt *Watcher) seconds() int {
	return int(max(wt.now.Sub(wt.start), 0) / time.Second)
}

func (wt *Watcher) columns() int {
	if wt.cols == nil {
		return 0
	}
	return wt.cols()
}

// Close ends the watching: it wipes the status bar, all of it, if it is on
// the screen, so that the terminal is left on a clean line however the
// watching ended, and draws it no more. It does nothing the second time.
func (wt *Watcher) Close() error {
	wt.live = false
	return wt.write(wt.unpin())
}

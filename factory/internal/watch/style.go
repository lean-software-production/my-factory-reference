package watch

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/term"
)

// The escapes a terminal is written. Plain output has none of them.
const (
	reset  = "\x1b[0m"
	bold   = "\x1b[1m"
	dim    = "\x1b[2m"
	red    = "\x1b[31m"
	green  = "\x1b[32m"
	yellow = "\x1b[33m"
	wipe   = "\r\x1b[2K" // wipe the line the cursor is on
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

// name is the machine's name in its colour.
func (wt *Watcher) name(machine string) string {
	colour, ok := wt.colours[machine]
	if !ok {
		colour = palette[len(wt.colours)%len(palette)]
		wt.colours[machine] = colour
	}
	return colour + machine + reset
}

// label is the machine's name in its colour, padded to the column.
func (wt *Watcher) label(machine string) string {
	return wt.name(machine) + strings.Repeat(" ", max(wt.width-len(machine), 0))
}

// show writes a line: the plain one, or, on a terminal, the styled one,
// above the pinned line.
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

// unpin is what wipes the pinned line from the screen, if it is there.
func (wt *Watcher) unpin() string {
	if !wt.pinned {
		return ""
	}
	wt.pinned = false
	return wipe
}

// pin is the pinned line, written at the foot of the screen, with no line
// ending, so that the next line replaces it: the line, how far the run has
// got if it has said, and how long it has gone. It is empty once the run
// has ended.
func (wt *Watcher) pin() string {
	if !wt.live {
		return ""
	}
	text := "▶ " + wt.line
	if wt.said {
		text += "  " + wt.progress.String() + "  " + wt.progress.bar()
	}
	text += "  " + elapsed(wt.now.Sub(wt.start))
	// A line longer than the terminal would wrap, and could not be wiped.
	if cols := wt.columns(); cols > 0 && utf8.RuneCountInString(text) >= cols {
		text = string([]rune(text)[:cols-1])
	}
	wt.pinned, wt.drawn = true, wt.seconds()
	return bold + text + reset
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

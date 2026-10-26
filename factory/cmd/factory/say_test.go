package main

import (
	"path/filepath"
	"slices"
	"testing"
)

// say names the run, then the machine and the message, with the run's
// target given anywhere among them. After "--", a message can begin with
// "-".
func TestSayReadsTheRunTheMachineAndTheMessage(t *testing.T) {
	target, err := filepath.Abs("t")
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"tetris", "--target", "t", "doer", "use curses"},
		{"--target", "t", "tetris", "doer", "use curses"},
		{"tetris", "doer", "--target=t", "use curses"},
		{"tetris", "--target", "t", "--", "doer", "use curses"},
	} {
		i, rest, err := runArgs("say", args)
		if err != nil {
			t.Errorf("%q: %v", args, err)
			continue
		}
		if i.run != "tetris" || i.target != target || !slices.Equal(rest, []string{"doer", "use curses"}) {
			t.Errorf("%q was read as run %q, target %q, then %q", args, i.run, i.target, rest)
		}
	}
	for _, args := range [][]string{
		{"tetris", "--target", "t", "doer", "--", "-v please"},
		{"tetris", "--target", "t", "--", "doer", "-v please"},
	} {
		_, rest, err := runArgs("say", args)
		if err != nil || !slices.Equal(rest, []string{"doer", "-v please"}) {
			t.Errorf("%q: the machine and message were read as %q (%v)", args, rest, err)
		}
	}
}

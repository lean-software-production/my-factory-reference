package main

import (
	"os"
	"path/filepath"
	"testing"
)

// tetris007 holds the lines the course's Tetris runs used: the real graphs.
var tetris007 = filepath.Join("..", "..", "..", "tetris", "tetris-007", ".assembly-lines")

func routeOfFile(t *testing.T, dir string) *assemblyLine {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "assembly-line.dot"))
	if err != nil {
		t.Fatal(err)
	}
	l, err := parseAssemblyLine(raw)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// The route is written from start, in order, along the edges. A fan out is
// shown as its branches in brackets, and the edges that go back to a
// machine already written come last, after one arrow.
func TestTheRouteOfTheRealLines(t *testing.T) {
	for name, tc := range map[string]struct{ dir, want, planner string }{
		"implement-with-ralph-loop": {ralphLoop, "start ▸ planner ▸ ralph ↺ planner", "planner"},
		"quick":                     {filepath.Join(tetris007, "quick"), "start ▸ planner ▸ doer ↺ planner", "planner"},
		"careful":                   {filepath.Join(tetris007, "careful"), "start ▸ planner ▸ doer ▸ validator ↺ doer, planner", "planner"},
		"three-big-brains":          {filepath.Join(tetris007, "three-big-brains"), "start ▸ planner ▸ doer ▸ [reviewer_1 reviewer_2 reviewer_3] ▸ synthesiser ↺ doer, planner", "planner"},
	} {
		t.Run(name, func(t *testing.T) {
			route := routeOfFile(t, tc.dir).route()
			if got := route.String(); got != tc.want {
				t.Errorf("the route is\n  %s\nnot\n  %s", got, tc.want)
			}
			if route.Planner != tc.planner {
				t.Errorf("the planner is %q, not %q", route.Planner, tc.planner)
			}
		})
	}
}

// The planner is the machine with an edge to finish, whatever it is called.
func TestTheRoutesPlannerIsFoundFromTheGraph(t *testing.T) {
	l, err := parseAssemblyLine([]byte(`digraph {
  start -> lead
  lead -> worker [label="not complete"]
  lead -> finish [label="complete"]
  worker -> lead
}`))
	if err != nil {
		t.Fatal(err)
	}
	route := l.route()
	if route.Planner != "lead" {
		t.Errorf("the planner is %q, not lead", route.Planner)
	}
	if got, want := route.String(), "start ▸ lead ▸ worker ↺ lead"; got != want {
		t.Errorf("the route is %q, not %q", got, want)
	}
}

// A line that cannot be walked that way gives the machines in the order
// they first appear in the graph, and never panics.
func TestARouteThatCannotBeWalkedFallsBackToTheMachinesInOrder(t *testing.T) {
	for name, tc := range map[string]struct{ dot, want string }{
		"a machine only another branch reaches": {`digraph {
  start -> planner
  planner -> a [label="x"]
  planner -> b [label="y"]
  planner -> finish [label="complete"]
  a -> planner
  b -> c
  c -> planner
}`, "start ▸ planner ▸ a ▸ b ▸ c"},
		"branches that do not meet": {`digraph {
  start -> planner
  planner -> a
  planner -> b
  a -> c
  b -> d
  c -> planner
  d -> finish
}`, "start ▸ planner ▸ a ▸ b ▸ c ▸ d"},
		"branches that lead back": {`digraph {
  start -> planner
  planner -> a
  planner -> b
  a -> planner
  b -> planner
  planner -> finish [label="complete"]
}`, "start ▸ planner ▸ a ▸ b"},
		"no start": {`digraph { planner -> doer; doer -> planner; planner -> finish [label=complete] }`, "start ▸ planner ▸ doer"},
		"nothing":  {`digraph { }`, "start"},
	} {
		t.Run(name, func(t *testing.T) {
			l, err := parseAssemblyLine([]byte(tc.dot))
			if err != nil {
				t.Fatal(err)
			}
			if got := l.route().String(); got != tc.want {
				t.Errorf("the route is %q, not %q", got, tc.want)
			}
		})
	}
}

// A line that cannot be read gives no route; it does not stop watching.
func TestARouteOfALineThatCannotBeReadIsEmpty(t *testing.T) {
	if got := routeOf(t.TempDir()); got.String() != "start" || got.Planner != "" {
		t.Errorf("a line with no graph gave %q, planner %q", got.String(), got.Planner)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "assembly-line.dot"), []byte("not a graph {{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := routeOf(dir); got.String() != "start" {
		t.Errorf("a broken graph gave %q", got.String())
	}
	if got := routeOf(ralphLoop).String(); got != "start ▸ planner ▸ ralph ↺ planner" {
		t.Errorf("the implement-with-ralph-loop folder gave %q", got)
	}
}

// The target is shown from the folder watch is run in when it is in it.
func TestTheTargetIsShownFromWhereWatchIsRun(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for target, want := range map[string]string{
		filepath.Join(wd, "tetris", "release-ride-3"): filepath.Join("tetris", "release-ride-3"),
		wd:                       wd,
		filepath.Dir(wd):         filepath.Dir(wd),
		"/somewhere/else/entire": "/somewhere/else/entire",
	} {
		if got := shownTarget(target); got != want {
			t.Errorf("%s is shown as %q, not %q", target, got, want)
		}
	}
}

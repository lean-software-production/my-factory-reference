package main

import (
	"maps"
	"os"
	"slices"
	"testing"
)

func lineFrom(t *testing.T, path string) *assemblyLine {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	l, err := parseAssemblyLine(raw)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func ranksOf(all []placed) map[string]int {
	ranks := map[string]int{}
	for _, p := range all {
		ranks[p.Node] = p.Rank
	}
	return ranks
}

func rowsOf(all []placed, nodes ...string) []float64 {
	var rows []float64
	for _, node := range nodes {
		for _, p := range all {
			if p.Node == node {
				rows = append(rows, p.Row)
			}
		}
	}
	return rows
}

// backEdgesOf are the edges that go back, written "from->to".
func backEdgesOf(l *assemblyLine) []string {
	var back []string
	for i, isBack := range l.backEdges() {
		if isBack {
			back = append(back, l.edges[i].from+"->"+l.edges[i].to)
		}
	}
	return back
}

func checkLayout(t *testing.T, l *assemblyLine, wantRanks map[string]int, wantBack []string) []placed {
	t.Helper()
	placed := l.layout()
	if got := ranksOf(placed); !maps.Equal(got, wantRanks) {
		t.Errorf("ranks are %v, not %v", got, wantRanks)
	}
	if got := len(placed); got != len(l.nodes) {
		t.Errorf("%d nodes placed, not %d", got, len(l.nodes))
	}
	for i, p := range placed {
		if p.Node != l.nodes[i] {
			t.Errorf("node %d placed is %s, not %s: they must stay in the order the line has them", i, p.Node, l.nodes[i])
		}
	}
	if got := backEdgesOf(l); !slices.Equal(got, wantBack) {
		t.Errorf("back edges are %q, not %q", got, wantBack)
	}
	return placed
}

// Read in the order it is written, the demo line sends work to doer before
// it plans; the replay has to show it planning first.
func TestLayoutOfTheDemoLineReadsPlanningFirst(t *testing.T) {
	l := lineFrom(t, "../../../demo/implement-with-3-big-brains/.assembly-lines/implement-with-3-big-brains/assembly-line.dot")
	placed := checkLayout(t, l, map[string]int{
		"start": 0, "plan_gate": 1, "plan_author": 2,
		"plan_reviewer_opus": 3, "plan_reviewer_gemini": 3, "plan_reviewer_astra": 3,
		"plan_synthesiser": 4, "doer": 5, "validator": 6, "planner": 7, "finish": 8,
	}, []string{"plan_synthesiser->plan_author", "planner->doer", "validator->doer", "validator->plan_author"})
	if got, want := rowsOf(placed, "plan_reviewer_opus", "plan_reviewer_gemini", "plan_reviewer_astra"), []float64{-1, 0, 1}; !slices.Equal(got, want) {
		t.Errorf("the reviewers are on rows %v, not %v", got, want)
	}
}

func TestLayoutOfTheRalphLoop(t *testing.T) {
	l := lineFrom(t, "../../../lines/implement-with-ralph-loop/assembly-line.dot")
	checkLayout(t, l, map[string]int{"start": 0, "planner": 1, "ralph": 2, "finish": 3}, []string{"ralph->planner"})
}

func TestLayoutOfThreeBigBrains(t *testing.T) {
	l := lineFrom(t, "../../../tetris/tetris-007/.assembly-lines/three-big-brains/assembly-line.dot")
	placed := checkLayout(t, l, map[string]int{
		"start": 0, "planner": 1, "doer": 2,
		"reviewer_1": 3, "reviewer_2": 3, "reviewer_3": 3,
		"synthesiser": 4, "finish": 5,
	}, []string{"synthesiser->doer", "synthesiser->planner"})
	if got, want := rowsOf(placed, "reviewer_1", "reviewer_2", "reviewer_3"), []float64{-1, 0, 1}; !slices.Equal(got, want) {
		t.Errorf("the reviewers are on rows %v, not %v", got, want)
	}
	if got, want := rowsOf(placed, "doer"), []float64{0}; !slices.Equal(got, want) {
		t.Errorf("doer is on rows %v, not %v", got, want)
	}
}

func TestLayoutOfCareful(t *testing.T) {
	l := lineFrom(t, "../../../tetris/tetris-007/.assembly-lines/careful/assembly-line.dot")
	checkLayout(t, l, map[string]int{"start": 0, "planner": 1, "doer": 2, "validator": 3, "finish": 4},
		[]string{"validator->doer", "validator->planner"})
}

// A machine start never reaches is still drawn, so a hand-made line shows
// its mistake rather than hiding it.
func TestLayoutPlacesAMachineStartNeverReaches(t *testing.T) {
	l, err := parseAssemblyLine([]byte(`digraph { start -> a; a -> finish; b -> a }`))
	if err != nil {
		t.Fatal(err)
	}
	placed := checkLayout(t, l, map[string]int{"start": 0, "a": 1, "b": 1, "finish": 2}, nil)
	if got, want := rowsOf(placed, "a", "b"), []float64{-0.5, 0.5}; !slices.Equal(got, want) {
		t.Errorf("a and b are on rows %v, not %v", got, want)
	}
}

// Rows follow the order the line has the machines in, except that a machine
// start never reaches comes after the others in its rank.
func TestLayoutPutsAnUnreachableMachineAfterTheOthersInItsRank(t *testing.T) {
	l, err := parseAssemblyLine([]byte(`digraph { b -> a; start -> a; a -> c; start -> c; c -> finish }`))
	if err != nil {
		t.Fatal(err)
	}
	placed := l.layout()
	if got, want := ranksOf(placed)["b"], 1; got != want {
		t.Errorf("b is at rank %d, not %d", got, want)
	}
	if got, want := rowsOf(placed, "a", "b"), []float64{-0.5, 0.5}; !slices.Equal(got, want) {
		t.Errorf("a and b are on rows %v, not %v", got, want)
	}
}

// Where the route doubles back is what the replay draws as the line's loops.
func TestBackEdgesOfTheDemoLine(t *testing.T) {
	l := lineFrom(t, "../../../demo/implement-with-3-big-brains/.assembly-lines/implement-with-3-big-brains/assembly-line.dot")
	want := []string{"plan_synthesiser->plan_author", "planner->doer", "validator->doer", "validator->plan_author"}
	if got := backEdgesOf(l); !slices.Equal(got, want) {
		t.Errorf("back edges are %q, not %q", got, want)
	}
}

package main

import (
	"slices"
	"testing"
)

// The assembly line is read by a Graphviz library, so names and labels may
// be quoted, and a label may be one word or several.
func TestLineIsReadWhateverIsQuoted(t *testing.T) {
	l, err := parseAssemblyLine([]byte(`digraph {
		start -> "the planner"
		"the planner" -> doer [label="not complete"]
		"the planner" -> finish [label=complete]
		doer -> "the planner"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"start", "the planner", "doer", "finish"}; !slices.Equal(l.nodes, want) {
		t.Errorf("nodes are %q, not %q", l.nodes, want)
	}
	if got := l.routedFields("the planner"); !slices.Equal(got, []string{"complete"}) {
		t.Errorf("the planner is routed on %q", got)
	}
	for result, want := range map[string]string{`{"complete": true}`: "finish", `{"complete": false}`: "doer"} {
		_, res := lastResult(result)
		if got, err := l.next("the planner", res); err != nil || got != want {
			t.Errorf("%s goes to %q (%v), not %q", result, got, err, want)
		}
	}
}

func TestLineThatIsNotAWellFormedDigraphIsRefused(t *testing.T) {
	if _, err := parseAssemblyLine([]byte(`graph { start -- planner }`)); err == nil {
		t.Error("an undirected graph was accepted")
	}
	if _, err := parseAssemblyLine([]byte(`digraph { start -> }`)); err == nil {
		t.Error("a graph that does not parse was accepted")
	}
}

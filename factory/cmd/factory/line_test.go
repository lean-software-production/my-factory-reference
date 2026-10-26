package main

import (
	"os"
	"path/filepath"
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

// The planner is the machine with an edge to finish, whatever it is called,
// and no other machine may have one. What the other machines are for follows
// from the line too.
func TestRolesFollowFromTheGraphNotTheNames(t *testing.T) {
	line := lineIn(t, `digraph {
		start -> lead
		lead -> builder  [label="not complete"]
		lead -> finish   [label=complete]
		builder -> inspector
		inspector -> checker
		checker -> builder [label="not approved"]
		checker -> lead    [label=approved]
	}`, "lead", "builder", "inspector", "checker")

	if got := line.planner(); got != "lead" {
		t.Errorf("the planner is %q, not lead", got)
	}
	for node, want := range map[string]struct{ worker, sendsBack bool }{
		"lead": {false, false}, "builder": {true, false}, "inspector": {false, false}, "checker": {false, true},
	} {
		if got := (struct{ worker, sendsBack bool }{line.isWorker(node), line.canSendWorkBack(node)}); got != want {
			t.Errorf("%s is worker %v, sends work back %v; want %v", node, got.worker, got.sendsBack, want)
		}
	}
}

func TestOnlyOneMachineMayHaveAnEdgeToFinish(t *testing.T) {
	for name, graph := range map[string]string{
		"a second machine": `digraph { start -> planner; planner -> doer [label="not complete"]; planner -> finish [label=complete]; doer -> finish }`,
		"start itself":     `digraph { start -> finish; start -> planner; planner -> finish [label=complete] }`,
	} {
		dir := lineFolder(t, graph, "planner", "doer")
		if _, err := loadAssemblyLine(dir); err == nil {
			t.Errorf("%s: a line with two ways to finish was accepted", name)
		}
	}
	if _, err := loadAssemblyLine(lineFolder(t, `digraph { start -> lead; lead -> finish }`, "lead")); err != nil {
		t.Errorf("a line whose planner is called lead was refused: %v", err)
	}
}

// lineFolder makes a line's folder with the graph and a folder for each machine.
func lineFolder(t *testing.T, graph string, machines ...string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "assembly-line.dot"), []byte(graph), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range machines {
		if err := os.MkdirAll(filepath.Join(dir, "machines", name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func lineIn(t *testing.T, graph string, machines ...string) *assemblyLine {
	t.Helper()
	line, err := loadAssemblyLine(lineFolder(t, graph, machines...))
	if err != nil {
		t.Fatal(err)
	}
	return line
}

// nextEdge says which edge a result takes, by its place in the line's edges,
// so a replay can light the very edge a run went along; next is where it leads.
func TestNextEdgeIsTheIndexOfTheEdgeTaken(t *testing.T) {
	l, err := parseAssemblyLine([]byte(`digraph assembly_line {
		start -> plan_gate
		plan_gate -> doer             [label="pending"]
		plan_gate -> planner          [label="ready"]
		plan_gate -> plan_author      [label="not ready"]
		plan_author -> plan_reviewer_opus
		plan_reviewer_opus -> plan_synthesiser
		plan_synthesiser -> plan_author [label="not satisfied"]
		plan_synthesiser -> doer        [label="satisfied"]
		planner -> doer        [label="not complete"]
		planner -> finish      [label="complete"]
		doer -> validator
		validator -> doer      [label="retry"]
		validator -> plan_author [label="replan"]
		validator -> planner   [label="satisfied"]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	edgeIndex := func(from, to string) int {
		t.Helper()
		for i, e := range l.edges {
			if e.from == from && e.to == to {
				return i
			}
		}
		t.Fatalf("no edge from %s to %s", from, to)
		return -1
	}
	for _, c := range []struct {
		from, to, result string
	}{
		{"validator", "plan_author", `{"retry": false, "replan": true, "satisfied": false}`},
		{"plan_gate", "planner", `{"pending": false, "ready": true}`},
		{"plan_gate", "plan_author", `{"pending": false, "ready": false}`},
		{"doer", "validator", `{}`},
	} {
		_, res := lastResult(c.result)
		i, err := l.nextEdge(c.from, res)
		if err != nil || i != edgeIndex(c.from, c.to) {
			t.Errorf("%s with %s takes edge %d (%v), not %d", c.from, c.result, i, err, edgeIndex(c.from, c.to))
		}
		if to, err := l.next(c.from, res); err != nil || to != c.to {
			t.Errorf("%s with %s goes to %q (%v), not %q", c.from, c.result, to, err, c.to)
		}
	}

	_, res := lastResult(`{"pending": false}`)
	want := `the result of plan_gate has no field "ready", which the assembly line routes on`
	if _, err := l.nextEdge("plan_gate", res); err == nil || err.Error() != want {
		t.Errorf("a missing routed field is reported as %v, not %q", err, want)
	}
}

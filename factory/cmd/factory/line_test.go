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

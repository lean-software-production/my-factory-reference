package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/awalterschulze/gographviz"
)

// The assembly line is the route through the factory's machines: a Graphviz
// graph the factory reads before it does any work. start and finish mark
// where it begins and ends; every other node names a machine on the line,
// which has a folder of its own, machines/<name>/ beside the graph.
//
// An edge only routes. Its label names a field of the result of the machine
// it leaves: the edge is taken when that field is true, and one labelled
// "not" and the field's name when it is false. An edge with no label is taken
// whatever the result.
const (
	start  = "start"
	finish = "finish"

	// plannerName is the machine that decides whether there is more to do:
	// the only one with an edge to finish. The doer does the work, and the
	// validator checks it.
	plannerName   = "planner"
	doerName      = "doer"
	validatorName = "validator"
)

type edge struct {
	from, to string
	label    string // as written, without its quotes
}

// assemblyLine is a graph that has been read and checked.
type assemblyLine struct {
	name  string // its folder's name
	dir   string
	nodes []string // in the order they first appear
	edges []edge   // in the order they are written
}

// machinesOn are the machines on the line: every node but start and finish.
func (l *assemblyLine) machinesOn() []string {
	return slices.DeleteFunc(slices.Clone(l.nodes), func(node string) bool { return node == start || node == finish })
}

func (l *assemblyLine) machineDir(name string) string {
	return filepath.Join(l.dir, "machines", name)
}

// linesFolder finds assembly-lines/ in the factory's folder: next to the
// factory's binary, or in the folder above it (the launcher builds the
// binary into .build/). It is found from the binary, not from where the
// factory is run.
func linesFolder() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	candidates := []string{filepath.Join(dir, "assembly-lines"), filepath.Join(filepath.Dir(dir), "assembly-lines")}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("there is no assembly-lines folder: looked in %s", strings.Join(candidates, " and "))
}

// readAssemblyLine reads the factory's one assembly line from the folder
// assembly-lines/ holds, and refuses it unless the factory can run it.
func readAssemblyLine() (*assemblyLine, error) {
	root, err := linesFolder()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var folders []string
	for _, entry := range entries {
		if entry.IsDir() {
			folders = append(folders, entry.Name())
		}
	}
	if len(folders) != 1 {
		return nil, fmt.Errorf("the factory has one assembly line, but %s holds %d folders", root, len(folders))
	}
	return loadAssemblyLine(filepath.Join(root, folders[0]))
}

func loadAssemblyLine(dir string) (*assemblyLine, error) {
	graph := filepath.Join(dir, "assembly-line.dot")
	raw, err := os.ReadFile(graph)
	if err != nil {
		return nil, fmt.Errorf("could not read the assembly line: %v", err)
	}
	l, err := parseAssemblyLine(raw)
	if err != nil {
		return nil, fmt.Errorf("could not read the assembly line %s: %v", graph, err)
	}
	l.name, l.dir = filepath.Base(dir), dir
	if err := l.check(); err != nil {
		return nil, fmt.Errorf("the assembly line %s is refused: %w", graph, err)
	}
	return l, nil
}

func parseAssemblyLine(dot []byte) (*assemblyLine, error) {
	ast, err := gographviz.Parse(dot)
	if err != nil {
		return nil, err
	}
	graph := gographviz.NewGraph()
	if err := gographviz.Analyse(ast, graph); err != nil {
		return nil, err
	}
	if !graph.Directed {
		return nil, fmt.Errorf("it is not a digraph")
	}
	l := &assemblyLine{}
	addNode := func(node string) {
		if !slices.Contains(l.nodes, node) {
			l.nodes = append(l.nodes, node)
		}
	}
	for _, e := range graph.Edges.Edges {
		e := edge{unquote(e.Src), unquote(e.Dst), unquote(e.Attrs["label"])}
		addNode(e.from)
		addNode(e.to)
		l.edges = append(l.edges, e)
	}
	return l, nil
}

// unquote takes the quotes off a Graphviz identifier, if it has any.
func unquote(id string) string {
	if unquoted, err := strconv.Unquote(id); err == nil {
		return unquoted
	}
	return id
}

// check says why the factory cannot run the line, if it cannot: it names a
// machine the factory does not have, it breaks the planner's place on the
// line, or some machine on it has no way to finish.
func (l *assemblyLine) check() error {
	if !slices.Contains(l.nodes, start) {
		return fmt.Errorf("it has no %s", start)
	}
	for _, name := range l.machinesOn() {
		if info, err := os.Stat(l.machineDir(name)); err != nil || name != filepath.Base(name) || !info.IsDir() {
			return fmt.Errorf("the factory has no machine called %q (looked for %s)", name, l.machineDir(name))
		}
	}
	for _, e := range l.edges {
		if e.to == finish && e.from != plannerName {
			return fmt.Errorf("only the %s may have an edge to %s, and %s has one", plannerName, finish, e.from)
		}
	}
	var stuck []string
	for _, node := range l.nodes {
		if node != finish && !l.reaches(node, finish) {
			stuck = append(stuck, fmt.Sprintf("%s cannot be reached from %s", finish, node))
		}
	}
	if len(stuck) > 0 {
		return fmt.Errorf("%s", strings.Join(stuck, "; "))
	}
	return nil
}

// reaches says whether there is a path along the edges from one node to another.
func (l *assemblyLine) reaches(from, to string) bool {
	seen := map[string]bool{from: true}
	for queue := []string{from}; len(queue) > 0; queue = queue[1:] {
		for _, e := range l.edges {
			if e.from != queue[0] || seen[e.to] {
				continue
			}
			if e.to == to {
				return true
			}
			seen[e.to] = true
			queue = append(queue, e.to)
		}
	}
	return false
}

// routedFields are the fields of a machine's result that its edges name.
func (l *assemblyLine) routedFields(node string) []string {
	var fields []string
	for _, e := range l.edges {
		if field, _ := e.condition(); e.from == node && field != "" && !slices.Contains(fields, field) {
			fields = append(fields, field)
		}
	}
	return fields
}

// condition is the field an edge's label names, and whether the edge is
// taken when the field is false rather than true. An unlabelled edge names
// no field.
func (e edge) condition() (field string, whenFalse bool) {
	label := strings.TrimSpace(e.label)
	if rest, ok := strings.CutPrefix(label, "not "); ok {
		return strings.TrimSpace(rest), true
	}
	return label, false
}

// next is the node the line goes to from a machine, given its result: the
// first edge leaving it that the result takes.
func (l *assemblyLine) next(from string, res result) (string, error) {
	for _, e := range l.edges {
		if e.from != from {
			continue
		}
		field, whenFalse := e.condition()
		if field == "" {
			return e.to, nil
		}
		value, ok := res[field]
		if !ok {
			return "", fmt.Errorf("the result of %s has no field %q, which the assembly line routes on", from, field)
		}
		truth, ok := value.(bool)
		if !ok {
			return "", fmt.Errorf("the field %q of the result of %s is neither true nor false, which the assembly line routes on", field, from)
		}
		if truth != whenFalse {
			return e.to, nil
		}
	}
	return "", fmt.Errorf("no edge leaving %s is taken by its result", from)
}

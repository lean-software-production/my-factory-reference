package main

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/lean-software-production/my-factory-reference/factory/internal/watch"
)

// routeOf is the route of the assembly line in the folder, for watch to
// show. A line that cannot be read has the route of no machines: watching
// goes on without it.
func routeOf(lineDir string) watch.Route {
	raw, err := os.ReadFile(filepath.Join(lineDir, "assembly-line.dot"))
	if err != nil {
		return watch.Route{}
	}
	l, err := parseAssemblyLine(raw)
	if err != nil {
		return watch.Route{}
	}
	return l.route()
}

// route is the way a run goes along the line, written from start in order,
// following the edges:
//
//   - a machine with several edges out and no labels on them fans out; its
//     branches are one step, and the walk goes on from where they meet;
//   - an edge that goes back to a machine already written is not followed:
//     it is noted once, in the order they are met, and shown after the route;
//   - finish is not shown, nor are the edges into it.
//
// A line that cannot be walked so, because some machine is not reached by
// it, gives its machines in the order they first appear in the graph. The
// planner is the machine with an edge to finish, whatever it is called.
func (l *assemblyLine) route() watch.Route {
	route := watch.Route{Planner: l.planner()}
	var ok bool
	if route.Steps, route.Back, ok = l.walk(); !ok {
		route.Steps, route.Back = nil, nil
		for _, machine := range l.machinesOn() {
			route.Steps = append(route.Steps, []string{machine})
		}
	}
	return route
}

// walk is the steps from start and the machines it goes back to, and
// whether it reached every machine on the line.
func (l *assemblyLine) walk() (steps [][]string, back []string, ok bool) {
	written := map[string]bool{start: true}
	goBack := func(node string) {
		if !slices.Contains(back, node) {
			back = append(back, node)
		}
	}
	for node := start; ; {
		var next string
		if branches := l.branches(node); branches != nil {
			meet, ok := l.meeting(branches)
			if !ok {
				return nil, nil, false
			}
			for _, branch := range branches {
				if written[branch] || branch == finish {
					return nil, nil, false
				}
				written[branch] = true
			}
			steps = append(steps, branches)
			if meet == finish {
				break
			}
			if written[meet] {
				goBack(meet)
				break
			}
			next = meet
		} else {
			for _, e := range l.edgesFrom(node) {
				switch {
				case e.to == finish:
				case written[e.to]:
					goBack(e.to)
				case next == "":
					next = e.to
				}
			}
		}
		if next == "" {
			break
		}
		steps = append(steps, []string{next})
		written[next] = true
		node = next
	}
	for _, machine := range l.machinesOn() {
		if !written[machine] {
			return nil, nil, false
		}
	}
	return steps, back, true
}

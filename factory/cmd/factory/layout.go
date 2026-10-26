package main

import "slices"

// To replay a run the factory draws its assembly line as a graph, left to
// right: each machine in a column, a rank, by how far along the line it is,
// and in a row within its column. It is drawn the way the line is run, so
// the edges that send work back, which a plain reading of the graph in the
// order it is written would tangle, are found first and kept out of the
// ranking.

// placed is where a node goes: its column, and its row within the column.
// Rows are centred on 0 and one apart, so a column of two has rows -0.5 and
// 0.5.
type placed struct {
	Node string
	Rank int
	Row  float64
}

// layout places every node of the line, start and finish included, in the
// order the line has them.
//
// A node's rank is the longest path to it from start along the edges that
// don't go back. finish is the line's last column on its own. A machine that
// start never reaches is no part of the route, but is still drawn: in the
// first column after start, below the machines that are.
func (l *assemblyLine) layout() []placed {
	order := l.followRoute().finished
	reached := map[string]bool{}
	for _, node := range order {
		reached[node] = true
	}
	back := l.backEdges()

	// The walk finishes a node only after every node it leads to along edges
	// that don't go back, so the last to finish comes first on the route.
	rank := map[string]int{start: 0}
	for _, node := range slices.Backward(order) {
		for i, e := range l.edges {
			if e.from != node || back[i] || e.to == finish {
				continue
			}
			rank[e.to] = max(rank[e.to], rank[node]+1)
		}
	}
	last := 0
	for _, node := range l.nodes {
		if node == finish {
			continue
		}
		if !reached[node] {
			rank[node] = 1
		}
		last = max(last, rank[node])
	}
	rank[finish] = last + 1
	reached[finish] = true

	byRank := map[int][]string{}
	for _, reachable := range []bool{true, false} {
		for _, node := range l.nodes {
			if reached[node] == reachable {
				byRank[rank[node]] = append(byRank[rank[node]], node)
			}
		}
	}
	row := map[string]float64{}
	for _, nodes := range byRank {
		for i, node := range nodes {
			row[node] = float64(i) - float64(len(nodes)-1)/2
		}
	}

	out := make([]placed, 0, len(l.nodes))
	for _, node := range l.nodes {
		out = append(out, placed{node, rank[node], row[node]})
	}
	return out
}

// backEdges says, for each edge of the line in the order they are written,
// whether it goes back: it leads to a machine the route has already come
// through to get to the one it leaves. The route is found by following the
// edges from start, taking the last one written first, so that a line
// written to do the work before it plans is still read planning first.
// An edge to finish never goes back.
func (l *assemblyLine) backEdges() []bool {
	return l.followRoute().back
}

// walked is what following the route from start finds.
type walked struct {
	back     []bool   // for each edge, whether it goes back
	finished []string // the nodes reached, in the order the walk was done with them
}

// followRoute follows the edges from start depth first, the last written first.
// finish is where it stops: nothing leaves it.
//
// Walking in reverse written order is a rule of thumb that fits the course's
// lines (ralph-loop, three-big-brains, careful and implement-with-3-big-brains),
// not a proof: a gate whose skip edge to a later machine is written last
// would have the planning part ranked after that machine.
func (l *assemblyLine) followRoute() walked {
	w := walked{back: make([]bool, len(l.edges))}
	onRoute := map[string]bool{}
	seen := map[string]bool{}
	var visit func(node string)
	visit = func(node string) {
		seen[node], onRoute[node] = true, true
		if node != finish {
			for i := len(l.edges) - 1; i >= 0; i-- {
				e := l.edges[i]
				if e.from != node {
					continue
				}
				if onRoute[e.to] {
					w.back[i] = true
				} else if !seen[e.to] {
					visit(e.to)
				}
			}
		}
		onRoute[node] = false
		w.finished = append(w.finished, node)
	}
	if slices.Contains(l.nodes, start) {
		visit(start)
	}
	return w
}

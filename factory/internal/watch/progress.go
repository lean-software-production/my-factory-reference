package watch

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// barCells is how many cells wide a progress bar is.
const barCells = 16

// progress is how far a run has got, in tasks.
type progress struct{ done, total int }

// parseProgress reads the progress a result reports: {"done": 5, "total":
// 12}. It reads nothing from what is missing, not an object, not whole
// numbers, or has no task to do, and a done outside 0 to total is held
// to it.
func parseProgress(v any) (progress, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return progress{}, false
	}
	done, ok1 := wholeNumber(m["done"])
	total, ok2 := wholeNumber(m["total"])
	if !ok1 || !ok2 || total <= 0 {
		return progress{}, false
	}
	return progress{done: min(max(done, 0), total), total: total}, true
}

// wholeNumber is a number as the result carries it, as a JSON number
// parsed into any or as one made in Go; a fraction is cut to its whole.
func wholeNumber(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), n == n && n > -1e15 && n < 1e15
	case json.Number:
		f, err := n.Float64()
		return int(f), err == nil && f > -1e15 && f < 1e15
	}
	return 0, false
}

// String is the progress as it is told: task 5/12.
func (p progress) String() string { return fmt.Sprintf("task %d/%d", p.done, p.total) }

// bar draws the progress in barCells cells, filled for the part done.
func (p progress) bar() string {
	filled := p.done * barCells / p.total
	return strings.Repeat("█", filled) + strings.Repeat("░", barCells-filled)
}

// elapsed is a time gone, to the second: 40s, 6m 40s, 1h 5m 3s.
func elapsed(d time.Duration) string {
	s := int(max(d, 0) / time.Second)
	switch {
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm %ds", s/60, s%60)
	}
	return fmt.Sprintf("%dh %dm %ds", s/3600, s%3600/60, s%60)
}

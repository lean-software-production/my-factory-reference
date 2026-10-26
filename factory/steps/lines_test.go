package steps

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// The student's lines may hold runs once they have used them. A copy of a
// line in an example's target must start with none, or "a run named tetris"
// would already be started, with a plan.
func TestACopyOfALineHoldsNoRuns(t *testing.T) {
	line := t.TempDir()
	for _, file := range []string{"assembly-line.dot", "machines/doer/AGENTS.md", "runs/tetris/plan.md", "runs/tetris/run.json"} {
		path := filepath.Join(line, file)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := git(line, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	got, err := lineFiles(line)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"assembly-line.dot", "machines/doer/AGENTS.md"}; !slices.Equal(got, want) {
		t.Errorf("a copy of the line gets %v, not %v", got, want)
	}
}

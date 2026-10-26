package steps

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
)

// The assembly line is assembly-line.dot in the line's folder, in the
// target. Examples change it by editing its text.

func (w *world) registerLineSteps(sc *godog.ScenarioContext) {
	sc.Step(`^this assembly line:$`, func(line *godog.DocString) error {
		// The target gets the line, with the machines the student wrote,
		// and a run that executes it.
		if len(w.runs) == 0 {
			if err := w.addLine("careful", "careful"); err != nil {
				return err
			}
			if err := w.nameRun("tetris", "careful"); err != nil {
				return err
			}
		}
		return w.writeLine(w.currentLineDir(), line.Content+"\n")
	})
	sc.Step(`^the validator has been taken out(?: of the "([^"]*)" line)?, so the doer goes straight to the planner$`, func(line string) error {
		return w.editLine(line, func(line string) string {
			var kept []string
			for _, l := range strings.Split(strings.TrimRight(line, "\n"), "\n") {
				if !strings.Contains(l, "validator") {
					kept = append(kept, l)
				}
			}
			// The last line closes the graph.
			last := len(kept) - 1
			kept = append(kept[:last:last], "  doer -> planner", kept[last])
			return strings.Join(kept, "\n") + "\n"
		})
	})
	sc.Step(`^"([^"]*)" is misspelt "([^"]*)" throughout (?:the assembly line|the "([^"]*)" line)$`, func(right, wrong, line string) error {
		return w.editLine(line, func(graph string) string { return strings.ReplaceAll(graph, right, wrong) })
	})
	sc.Step(`^the edge from validator to planner has been taken out$`, func() error {
		edge := regexp.MustCompile(`(?m)^\s*validator\s*->\s*planner\b.*\n`)
		return w.editLine("", func(line string) string { return edge.ReplaceAllString(line, "") })
	})
	sc.Step(`^the edges from validator are labelled "([^"]*)" and "([^"]*)"$`, func(yes, no string) error {
		return w.editLine("", func(line string) string {
			var out []string
			for _, l := range strings.Split(line, "\n") {
				if strings.HasPrefix(strings.TrimSpace(l), "validator") {
					l = strings.Replace(l, `"not satisfied"`, `"`+no+`"`, 1)
					l = strings.Replace(l, `"satisfied"`, `"`+yes+`"`, 1)
				}
				out = append(out, l)
			}
			return strings.Join(out, "\n")
		})
	})

	sc.Step(`^the factory reads the assembly line$`, w.readsTheLine)
	sc.Step(`^it accepts it$`, func() error {
		if w.exitErr != nil || w.timedOut {
			return fmt.Errorf("the factory refused the assembly line (%v):\n%s", w.exitErr, w.output)
		}
		return nil
	})
	sc.Step(`^it refuses it$`, func() error {
		if w.exitErr == nil {
			return fmt.Errorf("the factory accepted the assembly line:\n%s", w.output)
		}
		// Refusing is reading, before any machine does any work.
		return w.calledTimes(0)
	})
	sc.Step(`^it reports that it has no machine called "([^"]*)"$`, func(name string) error {
		return w.reports(fmt.Sprintf("no machine called %q", name))
	})
	sc.Step(`^it reports that finish cannot be reached from validator$`, func() error {
		return w.reports("finish cannot be reached from validator")
	})
}

func lineFile(lineDir string) string { return filepath.Join(lineDir, "assembly-line.dot") }

func (w *world) writeLine(lineDir, text string) error {
	return os.WriteFile(lineFile(lineDir), []byte(text), 0o644)
}

// editLine changes the text of the named line's graph in the current target,
// or of the current run's line if none is named.
func (w *world) editLine(name string, change func(string) string) error {
	lineDir := w.currentLineDir()
	if name != "" {
		lineDir = w.lineDirIn(w.target, name)
	}
	raw, err := os.ReadFile(lineFile(lineDir))
	if err != nil {
		return err
	}
	return w.writeLine(lineDir, change(string(raw)))
}

// readsTheLine has the factory read the assembly line by running the run
// that executes it, with machines that do nothing: it reads the line before it does
// any work, so a line it refuses ends the run before any machine is called.
func (w *world) readsTheLine() error {
	return w.run()
}

package steps

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"
)

// Watching a run is the watch command, following it from another process.
// What it shows is the student's to lay out; these steps need each line a
// machine generated shown as "<machine> | <line>", the machines running now
// as "running <machine>, <machine>", and what has been spent as
// "tokens <machine> <in> in / <out> out   <provider> <n> ...   run <n>".
// The run's record is record.jsonl in its folder, an event a line.

func (w *world) registerWatchSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the doer says "([^"]*)", then "([^"]*)"$`, func(first, then string) {
		w.does.DoerSays, w.does.DoerThenSays = []string{first}, []string{then}
	})
	sc.Step(`^the reviewers run on Anthropic, Google and OpenAI models$`, func() error {
		for i, model := range []string{"anthropic/claude-test", "google/gemini-test", "openai/gpt-test"} {
			if err := w.setModel(reviewers[i], model); err != nil {
				return err
			}
			if err := w.giveLogin(reviewers[i]); err != nil {
				return err
			}
		}
		return nil
	})

	sc.Step(`^I (?:watch|am watching) the "([^"]*)" run$`, w.startWatching)
	sc.Step(`^I am watching the "([^"]*)" run as it runs$`, func(name string) error {
		if err := w.startedOrFail(name); err != nil {
			return err
		}
		return w.startWatching(name)
	})
	sc.Step(`^I read the "([^"]*)" run's record$`, w.readRecord)

	sc.Step(`^what I am watching shows that the "([^"]*)" run was stopped$`, func(name string) error {
		return w.watched(func(shown string) error {
			if !regexp.MustCompile(`(?m)^run ` + regexp.QuoteMeta(name) + ` stopped$`).MatchString(shown) {
				return fmt.Errorf("watching does not show the %q run stopped:\n%s", name, shown)
			}
			return nil
		})
	})
	sc.Step(`^I see "([^"]*)" from (the doer|reviewer_1, reviewer_2 and reviewer_3)$`, func(words, from string) error {
		machines := []string{"doer"}
		if from != "the doer" {
			machines = reviewers
		}
		return w.watched(func(shown string) error {
			for _, machine := range machines {
				if !slices.Contains(saidBy(generated(shown), machine), words) {
					return fmt.Errorf("watching does not show %q from %s:\n%s", words, machine, shown)
				}
			}
			return nil
		})
	})
	sc.Step(`^I do not see "([^"]*)"$`, func(words string) error {
		if w.watching == nil {
			return errors.New("nothing is being watched")
		}
		if shown := w.watching.output(); strings.Contains(shown, words) {
			return fmt.Errorf("watching shows %q:\n%s", words, shown)
		}
		return nil
	})
	sc.Step(`^it shows reviewer_1, reviewer_2 and reviewer_3 as running$`, func() error {
		return w.watched(func(shown string) error {
			running := runningLines(shown)
			if len(running) == 0 {
				return fmt.Errorf("watching shows nothing running:\n%s", shown)
			}
			now := running[len(running)-1]
			for _, reviewer := range reviewers {
				if !slices.Contains(now, reviewer) {
					return fmt.Errorf("watching shows %v running, not every reviewer:\n%s", now, shown)
				}
			}
			return nil
		})
	})
	sc.Step(`^I see what the planner generated$`, func() error {
		said, err := w.saidByCalls("planner")
		if err != nil {
			return err
		}
		if len(said) == 0 {
			return fmt.Errorf("the planner generated nothing\n%s", w.output)
		}
		return w.watched(func(shown string) error {
			if missing := missingFrom(said, saidBy(generated(shown), "planner")); missing != nil {
				return fmt.Errorf("watching does not show the planner's %q:\n%s", *missing, shown)
			}
			return nil
		})
	})

	sc.Step(`^it shows what every machine generated$`, func() error {
		calls, err := w.calls()
		if err != nil {
			return err
		}
		for _, c := range calls {
			said, err := w.saidByCalls(c.machine)
			if err != nil {
				return err
			}
			if missing := missingFrom(said, w.recordSaidBy(c.machine)); missing != nil {
				return fmt.Errorf("the record does not have the %s's %q", c.machine, *missing)
			}
		}
		return nil
	})
	sc.Step(`^it shows what every machine spent$`, func() error {
		spent, err := w.spentByMachine()
		if err != nil {
			return err
		}
		if len(spent) == 0 {
			return errors.New("no machine spent anything")
		}
		recorded := map[string]tokens{}
		for _, ev := range w.record {
			if ev.Kind == "usage" {
				t := recorded[ev.Machine]
				recorded[ev.Machine] = tokens{t.in + ev.InputTokens, t.out + ev.OutputTokens}
			}
		}
		for machine, t := range spent {
			if recorded[machine] != t {
				return fmt.Errorf("the record has the %s spending %+v, and it spent %+v", machine, recorded[machine], t)
			}
		}
		return nil
	})
	sc.Step(`^the record is in the run's folder in the target, and committed$`, func() error {
		record := filepath.Join(w.runDir(w.ran), "record.jsonl")
		rel, err := filepath.Rel(w.repo, record)
		if err != nil {
			return err
		}
		if _, err := git(w.repo, "ls-files", "--error-unmatch", rel); err != nil {
			return fmt.Errorf("the record %s is not committed: %v", rel, err)
		}
		return noUncommittedChanges(w.repo, w.runDir(w.ran))
	})

	sc.Step(`^it shows tokens in and tokens out for the (\w+)$`, func(machine string) error {
		spent, err := w.spentByMachine()
		if err != nil {
			return err
		}
		return w.watchedToTheEnd(func(shown string) error {
			var latest *tokensLine
			for _, l := range tokensLines(shown) {
				if l.machine == machine {
					latest = &l
				}
			}
			if latest == nil {
				return fmt.Errorf("watching shows no tokens for the %s:\n%s", machine, shown)
			}
			if !about(latest.in, spent[machine].in) || !about(latest.out, spent[machine].out) {
				return fmt.Errorf("watching shows the %s's tokens as %s in / %s out, and it spent %d in / %d out:\n%s",
					machine, latest.in, latest.out, spent[machine].in, spent[machine].out, shown)
			}
			return nil
		})
	})
	sc.Step(`^it shows a total for each of Anthropic, Google and OpenAI$`, func() error {
		bills, err := w.spentByProvider()
		if err != nil {
			return err
		}
		return w.watchedToTheEnd(func(shown string) error {
			last, err := lastTokensLine(shown)
			if err != nil {
				return err
			}
			for _, provider := range []string{"anthropic", "google", "openai"} {
				if shown, ok := last.bills[provider]; !ok || !about(shown, bills[provider]) {
					return fmt.Errorf("watching shows %s's total as %q, and %d was spent on it:\n%s", provider, shown, bills[provider], last.text)
				}
			}
			return nil
		})
	})
	sc.Step(`^it shows a total for the whole run$`, func() error {
		bills, err := w.spentByProvider()
		if err != nil {
			return err
		}
		total := 0
		for _, n := range bills {
			total += n
		}
		return w.watchedToTheEnd(func(shown string) error {
			last, err := lastTokensLine(shown)
			if err != nil {
				return err
			}
			if !about(last.run, total) {
				return fmt.Errorf("watching shows the run's total as %q, and %d was spent:\n%s", last.run, total, last.text)
			}
			return nil
		})
	})
	sc.Step(`^what I watched showed (the doer's tokens|the run's total) go up$`, func(what string) error {
		return w.watchedToTheEnd(func(shown string) error {
			var seen []float64
			for _, l := range tokensLines(shown) {
				switch {
				case what == "the run's total":
					seen = append(seen, count(l.run))
				case l.machine == "doer":
					seen = append(seen, count(l.in)+count(l.out))
				}
			}
			if len(seen) < 2 || !slices.IsSorted(seen) || seen[0] == seen[len(seen)-1] {
				return fmt.Errorf("watching did not show %s go up (%v):\n%s", what, seen, shown)
			}
			return nil
		})
	})
}

// A watcher is a watch command following a run, from a process of its own.
type watcher struct {
	cmd  *exec.Cmd
	mu   sync.Mutex
	out  bytes.Buffer
	done chan struct{} // closed when the command has exited, with err
	err  error
}

func (wt *watcher) Write(p []byte) (int, error) {
	wt.mu.Lock()
	defer wt.mu.Unlock()
	return wt.out.Write(p)
}

func (wt *watcher) output() string {
	wt.mu.Lock()
	defer wt.mu.Unlock()
	return wt.out.String()
}

// wait waits until the watch command has ended, as it does when the run
// has, and fails unless it said the run finished.
func (wt *watcher) wait(limit time.Duration) error {
	select {
	case <-wt.done:
		return wt.err
	case <-time.After(limit):
		return fmt.Errorf("watching had not ended after %v", limit)
	}
}

func (wt *watcher) stop() {
	select {
	case <-wt.done:
	default:
		wt.cmd.Process.Kill()
		<-wt.done
	}
}

// startWatching starts the watch command on the run.
func (w *world) startWatching(name string) error {
	r, err := w.runNamed(name)
	if err != nil {
		return err
	}
	if w.watching != nil {
		w.watching.stop()
	}
	cmd := exec.Command(filepath.Join(w.repo, "bin", "factory"), "watch", name, "--target", r.target)
	cmd.Dir, cmd.Env = w.repo, w.environment()
	wt := &watcher{cmd: cmd, done: make(chan struct{})}
	cmd.Stdout, cmd.Stderr = wt, wt
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		wt.err = cmd.Wait()
		close(wt.done)
	}()
	w.watching = wt
	return nil
}

// watched waits until what is being watched passes the check.
func (w *world) watched(check func(shown string) error) error {
	if w.watching == nil {
		return errors.New("nothing is being watched")
	}
	return waitFor(waitLimit, func() error { return check(w.watching.output()) })
}

// watchedToTheEnd checks what watching showed, once the run has ended.
func (w *world) watchedToTheEnd(check func(shown string) error) error {
	if w.watching == nil {
		return errors.New("nothing is being watched")
	}
	w.watching.wait(doubleRunLimit)
	return check(w.watching.output())
}

// line is a line a machine generated, as watching shows it.
type line struct{ machine, text string }

var generatedLine = regexp.MustCompile(`^(\S+)\s+\| (.*)$`)

// generated are the lines watching showed machines generate, in order.
func generated(shown string) []line {
	var lines []line
	for _, l := range strings.Split(shown, "\n") {
		if m := generatedLine.FindStringSubmatch(l); m != nil {
			lines = append(lines, line{m[1], strings.TrimSpace(m[2])})
		}
	}
	return lines
}

func saidBy(lines []line, machine string) []string {
	var said []string
	for _, l := range lines {
		if l.machine == machine {
			said = append(said, l.text)
		}
	}
	return said
}

// missingFrom is the first of the lines that is not among those shown.
func missingFrom(lines, shown []string) *string {
	for _, l := range lines {
		if !slices.Contains(shown, strings.TrimSpace(l)) {
			return &l
		}
	}
	return nil
}

var runningLine = regexp.MustCompile(`^running\s+(.+)$`)

// runningLines are the machines watching showed running, each time it did.
func runningLines(shown string) [][]string {
	var running [][]string
	for _, l := range strings.Split(shown, "\n") {
		if m := runningLine.FindStringSubmatch(l); m != nil {
			running = append(running, strings.Split(m[1], ", "))
		}
	}
	return running
}

// tokensLine is a line of what has been spent, as watching shows it.
type tokensLine struct {
	text    string
	machine string
	in, out string
	bills   map[string]string // by provider
	run     string
}

var tokensPattern = regexp.MustCompile(`^tokens\s+(\S+) (\S+) in / (\S+) out((?:\s{2,}\S+ \S+)+)$`)

func tokensLines(shown string) []tokensLine {
	var lines []tokensLine
	for _, text := range strings.Split(shown, "\n") {
		m := tokensPattern.FindStringSubmatch(text)
		if m == nil {
			continue
		}
		l := tokensLine{text: text, machine: m[1], in: m[2], out: m[3], bills: map[string]string{}}
		for _, total := range regexp.MustCompile(`\s{2,}`).Split(strings.TrimSpace(m[4]), -1) {
			name, n, _ := strings.Cut(total, " ")
			if name == "run" {
				l.run = n
			} else {
				l.bills[strings.ToLower(name)] = n
			}
		}
		lines = append(lines, l)
	}
	return lines
}

func lastTokensLine(shown string) (tokensLine, error) {
	lines := tokensLines(shown)
	if len(lines) == 0 {
		return tokensLine{}, fmt.Errorf("watching shows no tokens:\n%s", shown)
	}
	return lines[len(lines)-1], nil
}

// count reads a number of tokens as watching shows it: 950, 12.4k or 3.1M.
func count(shown string) float64 {
	n, unit := shown, 1.0
	switch {
	case strings.HasSuffix(shown, "k"):
		n, unit = strings.TrimSuffix(shown, "k"), 1000
	case strings.HasSuffix(shown, "M"):
		n, unit = strings.TrimSuffix(shown, "M"), 1_000_000
	}
	f, err := strconv.ParseFloat(n, 64)
	if err != nil {
		return math.NaN()
	}
	return f * unit
}

// about says whether the number shown is n, to the precision it is shown
// in: 12.4k is any number from 12,350 to 12,450.
func about(shown string, n int) bool {
	digits := strings.TrimRight(shown, "kM")
	precision := count("1" + shown[len(digits):])
	if i := strings.Index(digits, "."); i >= 0 {
		precision /= math.Pow(10, float64(len(digits)-i-1))
	}
	return math.Abs(count(shown)-float64(n)) <= precision/2+1e-9
}

type tokens struct{ in, out int }

// spentByMachine is what the doubles reported each machine spent.
func (w *world) spentByMachine() (map[string]tokens, error) {
	raw, err := os.ReadFile(filepath.Join(w.doubleDir, "usage.log"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	spent := map[string]tokens{}
	for _, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var n, in, out int
		var machine string
		if _, err := fmt.Sscanf(l, "%d %s %d %d", &n, &machine, &in, &out); err == nil {
			t := spent[machine]
			spent[machine] = tokens{t.in + in, t.out + out}
		}
	}
	return spent, nil
}

// spentByProvider is what the doubles reported was spent on each
// provider's bill: each machine's model names its provider.
func (w *world) spentByProvider() (map[string]int, error) {
	spent, err := w.spentByMachine()
	if err != nil {
		return nil, err
	}
	bills := map[string]int{}
	for machine, t := range spent {
		config, err := w.config(w.currentLineDir(), machine)
		if err != nil {
			return nil, err
		}
		provider, _, _ := strings.Cut(fmt.Sprint(config["model"]), "/")
		bills[provider] += t.in + t.out
	}
	return bills, nil
}

// saidByCalls is every line the doubles generated as the machine in that
// folder.
func (w *world) saidByCalls(machine string) ([]string, error) {
	raw, err := os.ReadFile(filepath.Join(w.doubleDir, "calls.log"))
	if err != nil {
		return nil, err
	}
	var said []string
	for n, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if fields := strings.Fields(l); len(fields) != 3 || fields[2] != machine {
			continue
		}
		file, err := os.Open(filepath.Join(w.doubleDir, "said", fmt.Sprintf("%d.txt", n+1)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		lines := bufio.NewScanner(file)
		for lines.Scan() {
			said = append(said, lines.Text())
		}
		file.Close()
	}
	return said, nil
}

// recordEvent is the part of an event in a run's record the steps read.
type recordEvent struct {
	Kind         string `json:"kind"`
	Machine      string `json:"machine"`
	Text         string `json:"text"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
}

// readRecord reads the run's record, from its folder.
func (w *world) readRecord(name string) error {
	r, err := w.runNamed(name)
	if err != nil {
		return err
	}
	w.ran = r
	raw, err := os.ReadFile(filepath.Join(w.runDir(r), "record.jsonl"))
	if err != nil {
		return fmt.Errorf("the %q run has no record in its folder: %v", name, err)
	}
	w.record = nil
	for n, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var ev recordEvent
		if err := json.Unmarshal([]byte(l), &ev); err != nil {
			return fmt.Errorf("line %d of the record is not an event: %v", n+1, err)
		}
		w.record = append(w.record, ev)
	}
	return nil
}

// recordSaidBy is every line the record has the machine generating.
func (w *world) recordSaidBy(machine string) []string {
	var said []string
	for _, ev := range w.record {
		if ev.Kind == "text" && ev.Machine == machine {
			said = append(said, ev.Text)
		}
	}
	return said
}

package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/cucumber/godog"
	"github.com/hinshun/vt10x"
)

// The terminal a game is started in: the seed asks it to fit in 24 rows.
const (
	screenCols = 80
	screenRows = 24
	// How long a game gets to draw its board.
	startUpTime = 8 * time.Second
	// The fewest rows a Tetris board can have and still be one.
	minBoardRows = 10
)

func (w *world) registerRealAgentSteps(sc *godog.ScenarioContext) {
	sc.Step(`^a seed describing Tetris on a board (\d+) columns wide, started with "([^"]*)"$`,
		func(cols int, command string) error {
			return w.writeSeed(fmt.Sprintf(`# Tetris

Build a game of Tetris that runs in the terminal. Its board is %d columns wide.

Start it with:

    %s

Keep the complete game display within 24 terminal rows, including the board, score, controls, borders, and game-over messages.

You may install packages if they help you render or control the terminal display.
`, cols, command))
		})
	sc.Step(`^"([^"]*)" in the target starts Tetris$`, w.startsTetris)
	sc.Step(`^Tetris has been built in the target$`, func() error { return w.startsTetris("npm start") })
	sc.Step(`^its board is (\d+) columns wide$`, func(cols int) error {
		if w.board == nil {
			return fmt.Errorf("no board has been seen")
		}
		if !slices.Contains(w.board.plausibleColumns(), cols) {
			return fmt.Errorf("the board is %d characters wide inside, which is not %d columns:\n%s",
				w.board.width(), cols, w.screen)
		}
		return nil
	})
	sc.Step(`^every task in the plan comes from the seed$`, w.planComesFromSeed)
}

// startsTetris starts the game in a terminal the size the seed asks for,
// and checks it draws a Tetris board and keeps running.
func (w *world) startsTetris(command string) error {
	args := strings.Fields(command)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = w.targetDir()
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: screenRows, Cols: screenCols})
	if err != nil {
		return fmt.Errorf("starting %q: %v", command, err)
	}
	screen := vt10x.New(vt10x.WithSize(screenCols, screenRows), vt10x.WithWriter(terminal))
	drawn := make(chan struct{})
	go func() {
		io.Copy(screen, terminal) // until the game's side of the terminal closes
		close(drawn)
	}()

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	var exitErr error
	running := true
	select {
	case exitErr = <-exited:
		running = false
	case <-time.After(startUpTime):
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-exited
	}
	select {
	case <-drawn:
	case <-time.After(time.Second):
	}
	terminal.Close()

	screen.Lock()
	grid := screenGrid(screen)
	screen.Unlock()
	w.screen = strings.Join(gridLines(grid), "\n")
	if !running {
		return fmt.Errorf("%q exited (%v) instead of running the game:\n%s", command, exitErr, w.screen)
	}
	w.board = findBoard(grid)
	if w.board == nil {
		return fmt.Errorf("%q drew no Tetris board:\n%s", command, w.screen)
	}
	return nil
}

func screenGrid(view vt10x.View) [][]rune {
	cols, rows := view.Size()
	grid := make([][]rune, rows)
	for y := range rows {
		grid[y] = make([]rune, cols)
		for x := range cols {
			grid[y][x] = view.Cell(x, y).Char
			if grid[y][x] == 0 {
				grid[y][x] = ' '
			}
		}
	}
	return grid
}

func gridLines(grid [][]rune) []string {
	lines := make([]string, len(grid))
	for y, row := range grid {
		lines[y] = strings.TrimRight(string(row), " ")
	}
	return lines
}

// board is a Tetris well found on the screen: the rows its side walls run
// down, and the columns of those walls.
type board struct {
	top, bottom int // the rows the walls share, inclusive
	left, right int // the walls' columns
	grid        [][]rune
}

func (b *board) width() int { return b.right - b.left - 1 }

// The characters a well's side walls are drawn with.
const wallRunes = "|│┃║▏▕▌▐!█"

// Characters that mark an empty cell.
const emptyRunes = " .·"

type wallRun struct{ col, top, bottom int }

// findBoard finds the innermost pair of side walls, at least minBoardRows
// tall, with the most rows in common.
func findBoard(grid [][]rune) *board {
	var walls []wallRun
	for x := range grid[0] {
		start := -1
		for y := 0; y <= len(grid); y++ {
			isWall := y < len(grid) && strings.ContainsRune(wallRunes, grid[y][x])
			if isWall && start < 0 {
				start = y
			}
			if !isWall && start >= 0 {
				if y-start >= minBoardRows {
					walls = append(walls, wallRun{x, start, y - 1})
				}
				start = -1
			}
		}
	}

	var best *board
	for i, l := range walls {
		for _, r := range walls[i+1:] {
			if r.col <= l.col+1 {
				continue
			}
			top, bottom := max(l.top, r.top), min(l.bottom, r.bottom)
			if bottom-top+1 < minBoardRows || wallBetween(walls, l, r, top, bottom) {
				continue
			}
			candidate := &board{top: top, bottom: bottom, left: l.col, right: r.col, grid: grid}
			if best == nil || candidate.bottom-candidate.top > best.bottom-best.top {
				best = candidate
			}
		}
	}
	return best
}

// wallBetween says whether another wall stands between l and r over the
// rows they share: then they enclose more than a well.
func wallBetween(walls []wallRun, l, r wallRun, top, bottom int) bool {
	for _, m := range walls {
		if m.col > l.col && m.col < r.col && min(m.bottom, bottom)-max(m.top, top)+1 >= minBoardRows {
			return true
		}
	}
	return false
}

// plausibleColumns is how many columns the board could have: one per
// character inside the walls, or one per two characters if nothing on the
// board says its cells are one character wide.
func (b *board) plausibleColumns() []int {
	w := b.width()
	columns := []int{w}
	if w%2 == 0 && !b.hasSingleWidthCells() {
		columns = append(columns, w/2)
	}
	return columns
}

// hasSingleWidthCells looks for a block or empty-cell mark that starts or
// ends on an odd character inside the walls.
func (b *board) hasSingleWidthCells() bool {
	dotParity := -1
	for y := b.top; y <= b.bottom; y++ {
		inside := b.grid[y][b.left+1 : b.right]
		for x := 0; x < len(inside); x++ {
			if inside[x] == '.' || inside[x] == '·' {
				if dotParity >= 0 && x%2 != dotParity {
					return true
				}
				dotParity = x % 2
				continue
			}
			if strings.ContainsRune(emptyRunes, inside[x]) {
				continue
			}
			start := x
			for x+1 < len(inside) && !strings.ContainsRune(emptyRunes, inside[x+1]) {
				x++
			}
			if start%2 != 0 || (x-start+1)%2 != 0 {
				return true
			}
		}
	}
	return false
}

// planComesFromSeed asks pi to judge whether each task in the plan is part
// of building what the seed describes.
func (w *world) planComesFromSeed() error {
	seed, err := os.ReadFile(w.seedPath())
	if err != nil {
		return err
	}
	plan, err := os.ReadFile(w.planPath())
	if err != nil {
		return err
	}
	return w.judge(fmt.Sprintf(`You are checking a plan against the seed it was made from.

The seed:
<<<
%s
>>>

The plan:
<<<
%s
>>>

Is every task in the plan a step towards building what the seed describes, with no task for anything the seed doesn't ask for or imply? Setting up the project, tests and the like count as steps towards it.`, seed, plan),
		"not every task comes from the seed")
}

// judge asks pi a yes or no question, and fails with the reason it gives
// unless the answer is yes.
func (w *world) judge(question, otherwise string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	judge := exec.CommandContext(ctx, "pi", "-p", "--no-tools", "--no-session", question+`

Answer with one line of JSON and nothing else: {"verdict": true or false, "reason": "<one sentence>"}`)
	judge.Dir = w.ws
	judge.Stderr = io.Discard
	out, err := judge.Output()
	if err != nil {
		return fmt.Errorf("asking pi to judge: %v\n%s", err, out)
	}
	var verdict struct {
		Verdict *bool  `json:"verdict"`
		Reason  string `json:"reason"`
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if json.Unmarshal([]byte(strings.TrimSpace(lines[i])), &verdict) == nil && verdict.Verdict != nil {
			break
		}
	}
	if verdict.Verdict == nil {
		return fmt.Errorf("pi gave no verdict:\n%s", out)
	}
	if !*verdict.Verdict {
		return fmt.Errorf("%s: %s", otherwise, verdict.Reason)
	}
	return nil
}

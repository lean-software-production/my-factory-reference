# visualise-run Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A hidden `factory visualise-run <record.jsonl>` command that serves a page replaying a finished run: the run's assembly line drawn as a hand-drawn graph, its machines lighting up as they work, their logs in speech bubbles, and a bar to play, speed up, seek and skip.

**Architecture:** All the replay's logic is in Go, in package `main` beside the line code it reuses: a layout of the graph (ranks and rows) and a replay document built from the record (frames of state at each moment, hand-offs, log lines, breaks, the end). The command serves that document as `run.json`, with a page of plain HTML, CSS and JavaScript embedded in the binary from a new package `internal/replayweb`, which also holds a vendored copy of the brand kit and its fonts. The page only draws: it picks the last frame at or before the moment shown.

**Tech Stack:** Go 1.27 (`net/http`, `embed`, `encoding/json`), plain ES2020 JavaScript and SVG, no build step. Playwright 1.64.1 with Chromium for the browser check only.

**Spec:** `docs/superpowers/specs/2026-10-26-visualise-run-design.md`. Mockups beside it: `2026-10-26-visualise-run-mockup-efficient.html` (the demo line: layout, models, result lines, the timeline's break) and `2026-10-26-visualise-run-mockup.html` (bubble placement and tails; ignore its flashing).

## Global Constraints

- The command is **hidden**: not in `factory --help`, not in any usage text, not in `main.go`'s package comment's command list, not in the starter's README.
- **No network** from the page: no CDN, no web fonts, no scripts from elsewhere. Everything is embedded in the binary.
- **No new tools** for students: no Graphviz, no Node at run time. The release stays one static binary per platform (`CGO_ENABLED=0`, as `bin/release` builds).
- **One parser:** the graph is read with `parseAssemblyLine` (`cmd/factory/line.go`). Routing reuses the line's own rules (`next`, `branches`). The command does not call `check()`: demo folders have no machine folders.
- The replay depends **only on the record and the graph**, never on the plan or on any machine's prompt.
- **Framing:** no response carries `X-Frame-Options` or a `Content-Security-Policy` (with or without `frame-ancestors`); no page has a `<meta http-equiv>`; no script breaks out of a frame or refuses to run in one.
- The server answers **any `Host`** and checks no `Origin`. The page uses **relative addresses only**.
- No pop-up windows, no `alert` or `confirm`, no storage the page needs to work. Every control works with a click; keyboard shortcuts are only a bonus.
- **Brand:** only the kit's colours and fonts (`sketchbook.css` variables and roles). Working machine: steady fill `color-mix(in srgb, var(--sk-mustard) 45%, var(--sk-paper))`, outline `var(--sk-mustard-text)`, no flashing. Done: `color-mix(in srgb, var(--sk-forest) 14%, var(--sk-paper))` and a ✓ after the name. Back edges `var(--sk-blue)`, others `var(--sk-slate)`. Untaken edges dotted. `start` and `finish` are pills with dashed outlines. `finish` lights in forest when the run finishes.
- Times shown as `watch` shows them: `Ns`, `Nm Ns`, `Nh Nm Ns`. Log stamps `m:ss` (`h:mm:ss` from an hour). Tokens as `watch.Count` gives them, then ` tokens`.
- `watch` and the feature suite don't change. `cd factory && go test ./... -count=1`, `go vet ./...` and `gofmt -l .` stay clean throughout.
- Commits end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Work on a branch `visualise-run`, never `main`. Don't push or release unless David asks.

## Review Focus

1. **A record that ends while a line is half-written** (the factory still appending): the last line without a newline is ignored, not an error. Owned by Task 5 (`TestReadRecordIgnoresAPartialLastLine`).
2. **A machine whose turn ends in an error** (the demo's planner at 17:25, `release-ride`'s doer): it hands off to no one, leaves `working`, and isn't marked done. Owned by Task 3 (`TestAnErrorEndsATurnWithoutAHandOff`) and Task 4 (`TestAFailedTurnCreditsNoEdge`).
3. **Very long machine text** (the plan author's 500-character result, a doer's 900-character line): bubbles wrap and scroll; result lines are cut to 80 characters plus `…`. Owned by Task 4 (`TestResultLineIsShortAndReadable`) and Task 7's browser check (bubble stays inside a 640px frame).
4. **Machine text containing HTML or control characters**: shown as text, never as markup. Owned by Task 4 (`TestLogLinesStripControlCharacters`) and Task 7 (the page sets `textContent`, never `innerHTML`, for machine text; the browser check feeds a `<b>` and sees it literally).
5. **A record whose run has a name or line different from its folder** (David's files kept elsewhere, `--graph`): names come from the path and `run.json`; a missing `run.json` is fine. Owned by Task 5 (`TestNamesComeFromThePathWithoutRunJSON`).

---

### Task 1: Lay out the graph

**Files:**
- Create: `factory/cmd/factory/layout.go`
- Test: `factory/cmd/factory/layout_test.go`

**Interfaces:**
- Consumes: `parseAssemblyLine([]byte) (*assemblyLine, error)`, `assemblyLine.nodes`, `.edges`, constants `start`, `finish` (`line.go`).
- Produces:
  - `type placed struct { Node string; Rank int; Row float64 }`
  - `func (l *assemblyLine) layout() []placed` — one per node, `start` and `finish` included, in `l.nodes` order.
  - `func (l *assemblyLine) backEdges() []bool` — one per `l.edges`, true for an edge that goes back.

**The algorithm** (the spec requires planning first on the demo line, and a plain DFS in written order gets it wrong):
1. Back edges: depth-first from `start`, visiting each node's out-edges in **reverse written order**. An edge to a node on the current DFS path is a back edge. (Edges to `finish` are never back.)
2. Rank: longest path from `start` (rank 0) over the edges that aren't back. Then `finish` alone takes the last column: `max rank of every other node + 1`.
3. Row: within a rank, nodes in `l.nodes` order get rows centred on 0, one apart: 1 node → `0`; 2 → `-0.5, 0.5`; 3 → `-1, 0, 1`.
4. A node that the DFS never reaches goes to rank 1, after the others in its rank. Edges from such nodes don't count in step 2's longest paths.

- [ ] **Step 1: Write the failing tests**, reading the real graphs from the repo:

```go
func TestLayoutOfTheDemoLineReadsPlanningFirst(t *testing.T) {
	l := lineFrom(t, "../../../demo/efficient/.assembly-lines/efficient/assembly-line.dot")
	ranks := ranksOf(l.layout()) // map[string]int
	want := map[string]int{"start": 0, "plan_gate": 1, "plan_author": 2,
		"plan_reviewer_opus": 3, "plan_reviewer_gemini": 3, "plan_reviewer_astra": 3,
		"plan_synthesiser": 4, "doer": 5, "validator": 6, "planner": 7, "finish": 8}
	// assert ranks == want
	// rows of rank 3, in order opus, gemini, astra: -1, 0, 1
}

func TestBackEdgesOfTheDemoLine(t *testing.T) {
	// edges reported back, as "from->to": exactly
	// plan_synthesiser->plan_author, planner->doer, validator->doer, validator->plan_author
}

func TestLayoutOfTheRalphLoop(t *testing.T) {
	// ../../../lines/implement-with-ralph-loop/assembly-line.dot:
	// start 0, planner 1, ralph 2, finish 3; back edges exactly ralph->planner
}

func TestLayoutOfThreeBigBrains(t *testing.T) {
	// ../../../tetris/tetris-007/.assembly-lines/three-big-brains/assembly-line.dot:
	// start 0, planner 1, doer 2, reviewer_1/2/3 at 3 with rows -1, 0, 1, synthesiser 4, finish 5;
	// back edges exactly synthesiser->doer, synthesiser->planner
}

func TestLayoutOfCareful(t *testing.T) {
	// tetris-007 careful: start 0, planner 1, doer 2, validator 3, finish 4;
	// back edges exactly validator->doer, validator->planner
}

func TestLayoutPlacesAMachineStartNeverReaches(t *testing.T) {
	// digraph { start -> a; a -> finish; b -> a }: b at rank 1, after a (rows -0.5, 0.5), no panic
}
```

`lineFrom` reads the file and calls `parseAssemblyLine`. The demo graph doesn't exist until Step 3.

- [ ] **Step 2: Run them and see them fail**

Run: `cd factory && go test ./cmd/factory -run 'TestLayout|TestBackEdges' -count=1`
Expected: FAIL (`l.layout undefined`).

- [ ] **Step 3: Add the demo material** David provided, from BB thread storage `thr_ahpnztqd9i/demo-line/`: `assembly-line.dot` to `demo/efficient/.assembly-lines/efficient/assembly-line.dot` and `record.jsonl` to `demo/efficient/.assembly-lines/efficient/runs/tetris-007/record.jsonl`, unchanged. Add `demo/README.md`: two sentences saying this is a real run of the reference factory, kept for `factory visualise-run demo/efficient/.assembly-lines/efficient/runs/tetris-007/record.jsonl`, and that it isn't a target to build in.

- [ ] **Step 4: Implement `layout` and `backEdges` in `layout.go`** as above.

- [ ] **Step 5: Run the tests and see them pass**

Run: `cd factory && go test ./cmd/factory -count=1`
Expected: PASS, including every existing test.

- [ ] **Step 6: Commit**

```bash
git add demo factory/cmd/factory/layout.go factory/cmd/factory/layout_test.go
git commit -m "Lay out an assembly line's graph for replaying a run, planning first"
```

---

### Task 2: Which edge a result takes

**Files:**
- Modify: `factory/cmd/factory/line.go:276-300` (`next`)
- Test: `factory/cmd/factory/line_test.go`

**Interfaces:**
- Produces: `func (l *assemblyLine) nextEdge(from string, res result) (int, error)` — the index in `l.edges` of the edge `next` would take. `next` becomes a wrapper that returns `l.edges[i].to`, with the same errors as now.

- [ ] **Step 1: Write the failing test**

```go
func TestNextEdgeIsTheIndexOfTheEdgeTaken(t *testing.T) {
	// the demo line's text inline; validator with {"retry":false,"replan":true,"satisfied":false}
	// gives the index of validator->plan_author; plan_gate with {"pending":false,"ready":true}
	// gives plan_gate->planner; ralph->planner-style unlabelled edge with result {} gives its index;
	// a missing routed field gives the same error text next gives today
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `cd factory && go test ./cmd/factory -run TestNextEdge -count=1`
Expected: FAIL (`l.nextEdge undefined`).

- [ ] **Step 3: Implement `nextEdge`, and make `next` call it.**

- [ ] **Step 4: Run all the factory's tests**

Run: `cd factory && go test ./... -count=1`
Expected: PASS.

- [ ] **Step 5: Commit** — `git commit -m "Say which edge a machine's result takes, not only where it leads"`

---

### Task 3: The replay's frames

**Files:**
- Create: `factory/cmd/factory/replay.go`
- Test: `factory/cmd/factory/replay_test.go`
- Create: `factory/cmd/factory/testdata/replay/` (hand-made records, below)

**Interfaces:**
- Consumes: `layout()`, `backEdges()` (Task 1), `planner()`, `watch.Event`, `watch.Count`.
- Produces (JSON field names exactly as tagged; the page in Tasks 6–7 reads them):

```go
type replayNames struct{ Run, Line, Target string }

type replayDoc struct {
	Run      string        `json:"run"`
	Line     string        `json:"line"`
	Target   string        `json:"target"`
	Nodes    []replayNode  `json:"nodes"`
	Edges    []replayEdge  `json:"edges"`
	Total    float64       `json:"total"`    // seconds of run time, idle gaps cut out
	Frames   []replayFrame `json:"frames"`   // in time order; the first has At 0
	Handoffs []handoff     `json:"handoffs"` // Task 4
	Lines    []logLine     `json:"lines"`    // Task 4
	Breaks   []runBreak    `json:"breaks"`
	End      runEnd        `json:"end"`
}
type replayNode struct {
	Name  string  `json:"name"`
	Label string  `json:"label"` // underscores as spaces
	Kind  string  `json:"kind"`  // "start", "machine" or "finish"
	Rank  int     `json:"rank"`
	Row   float64 `json:"row"`
}
type replayEdge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Label    string `json:"label"`
	Back     bool   `json:"back"`
	SendBack bool   `json:"sendBack"` // back, and to a machine other than the planner
}
type replayFrame struct {
	At       float64           `json:"at"`
	Working  []string          `json:"working"` // never null
	Done     []string          `json:"done"`    // done in this round; never null
	Round    int               `json:"round"`
	Counts   []int             `json:"counts"`  // per edge, as Edges; Task 4 fills them
	Tokens   string            `json:"tokens"`  // watch.Count(input+output so far)
	Models   map[string]string `json:"models"`  // machine -> shown model, from its latest usage
	Finished bool              `json:"finished"`
}
type handoff struct {
	At       float64 `json:"at"`
	Machine  string  `json:"machine"`
	SendBack bool    `json:"sendBack"`
}
type logLine struct {
	At      float64 `json:"at"`
	Machine string  `json:"machine"`
	Text    string  `json:"text"`
	Result  bool    `json:"result"`
}
type runBreak struct {
	At     float64 `json:"at"`
	Reason string  `json:"reason"`
}
type runEnd struct {
	State  string  `json:"state"` // "finished", "stopped", "failed" or "unfinished"
	At     float64 `json:"at"`
	Reason string  `json:"reason"`
	Tokens string  `json:"tokens"`
}

func buildReplay(l *assemblyLine, events []watch.Event, names replayNames) replayDoc
func shownModel(model string) string
```

**Run time.** The record's `run` events split it into segments: each `started` begins one, and the next `stopped`, `failed` or `finished` ends it. A segment with no end runs to the record's last event. Run time is the time inside segments, added up, so idle gaps between them vanish. Every `At` is run time in seconds.

**Frames.** One frame at 0, then one after every event that changes something shown (events at the same instant can share one):
- `start` adds the machine to `Working`. If it's the planner, `Round` goes up by one and `Done` empties.
- `end` without an error removes it from `Working` and adds it to `Done`. With an error, it only removes it.
- `usage` adds `input_tokens + output_tokens` to the total and sets `Models[machine] = shownModel(model)` (a `usage` with a model and no tokens, like `plan_gate`'s, still sets the model).
- `run` `finished` sets `Finished` and empties `Working`. `stopped` or `failed` empties `Working`.

**Breaks.** Each segment end that has another segment after it is a break at its run time. Reason: `stopped`, or `failed: ` plus the first line of the run event's error, cut to 100 characters with `…`.

**End.** The last segment's end: its state, or `unfinished` if the record stops without one. `At` = `Total`. `Reason` as for breaks. `Tokens` = the final count.

`shownModel("openai-codex/gpt-6.1-sol:high")` = `gpt-6.1-sol · high`: drop everything up to the last `/`; a `:` suffix becomes ` · ` and the suffix.

- [ ] **Step 1: Write the failing tests** (`replayOf(t, recordPath, graphPath)` reads both and calls `buildReplay`; `frameAt(doc, seconds)` is the last frame with `At <= seconds`):

```go
const demoRecord = "../../../demo/efficient/.assembly-lines/efficient/runs/tetris-007/record.jsonl"
const demoGraph = "../../../demo/efficient/.assembly-lines/efficient/assembly-line.dot"

func TestDemoRunTimeCutsOutTheGap(t *testing.T) {
	d := replayOf(t, demoRecord, demoGraph)
	// d.Total within 0.01 of 2201.482
	// d.Breaks == one break, At within 0.01 of 287.528, Reason "stopped"
	// d.End.State "finished", d.End.At == d.Total, d.End.Tokens "613.8k";
	// the last frame has Finished true and empty Working
}

func TestDemoFrameAtNineAndAHalfMinutes(t *testing.T) {
	f := frameAt(replayOf(t, demoRecord, demoGraph), 570)
	// Working [doer]; Done [planner]; Round 4; Tokens "149.4k";
	// Models["doer"] "gpt-6.1-sol · low"; Models["plan_reviewer_astra"] "gpt-6-astra · high";
	// Models["plan_gate"] "gpt-6.1-sol"
}

func TestDemoThreeReviewersWorkAtOnce(t *testing.T) {
	// frameAt(d, 110).Working holds exactly the three plan reviewers
}

func TestAnErrorEndsATurnWithoutAHandOff(t *testing.T) {
	// frameAt(d, 287.0): planner not in Working and not in Done (its turn ended "the run was stopped")
}

func TestAFailedAndResumedRideIsOneRun(t *testing.T) {
	// ../../../tetris/release-ride/.assembly-lines/ralph-loop/runs/ride/record.jsonl and its line:
	// Total within 0.01 of 507.36; one break at 176.572 whose Reason starts
	// "failed: could not read the doer's result: no line of its answer is a JSON object:";
	// End.State "finished"; the last frame has Finished true and empty Working
}

func TestRoundsCarryOnAcrossAResume(t *testing.T) {
	// release-ride: the round in the last frame equals the number of planner starts in the record
}

func TestFramesStartAtZeroAndNeverGoBack(t *testing.T) {
	// for each of the four real records: Frames[0].At == 0, At never decreases,
	// Working and Done never nil, len(Counts) == len(Edges)
}

func TestShownModel(t *testing.T) {
	// "openai-codex/gpt-6.1-sol:high" -> "gpt-6.1-sol · high"; "openai-codex/gpt-6-sol" -> "gpt-6-sol";
	// "gpt-5" -> "gpt-5"
}

func TestNodesAndEdgesOfTheDemoLine(t *testing.T) {
	// node "plan_reviewer_opus" has Label "plan reviewer opus", Kind "machine", Rank 3, Row -1;
	// start Kind "start", finish Kind "finish"; edge validator->doer has Label "retry",
	// Back true, SendBack true; planner->doer Back true, SendBack true;
	// a ralph loop's ralph->planner is Back true, SendBack false
}
```

- [ ] **Step 2: Run them and see them fail**

Run: `cd factory && go test ./cmd/factory -run 'TestDemo|TestAnError|TestAFailed|TestRounds|TestFrames|TestShownModel|TestNodesAndEdges' -count=1`
Expected: FAIL (`buildReplay undefined`).

- [ ] **Step 3: Implement `buildReplay` (frames, breaks, end, nodes, edges, total) and `shownModel` in `replay.go`.** Leave `Handoffs` and `Lines` empty and `Counts` all zero for Task 4.

- [ ] **Step 4: Run the tests and see them pass**

Run: `cd factory && go test ./cmd/factory -count=1`
Expected: PASS.

- [ ] **Step 5: Commit** — `git commit -m "Build a run's replay from its record: the time, the machines and the rounds"`

---

### Task 4: Hand-offs, edge counts and log lines

**Files:**
- Modify: `factory/cmd/factory/replay.go`
- Modify: `factory/cmd/factory/replay_test.go`
- Create: `factory/cmd/factory/testdata/replay/send-backs.jsonl`

**Interfaces:**
- Consumes: `nextEdge` (Task 2), `branches`, everything in Task 3.
- Produces: `Frames[].Counts`, `Handoffs`, `Lines` filled; `func resultLine(l *assemblyLine, machine string, res map[string]any) string`.

**Which edges were taken.** Not in the record; it's worked out with the line's own routing:
- At a machine's `end` without an error, it hands off: if it fans out (`branches` isn't nil), along every edge out; otherwise along `nextEdge(machine, res)`, where `res` is that turn's `result` (or `{}` if the turn had none). A routing error means no hand-off. These edges become **pending**.
- At a `start` of M: if it's the first start of its segment and `start → M` exists, credit that edge. Otherwise credit every pending edge into M, and they stop being pending.
- At `run` `finished`, credit pending edges into `finish`.
- At a segment's end, pending edges are dropped.
- Crediting adds one to that edge's count in the next frame. Each `start` is a hand-off; it's a send-back if any edge credited at it is `SendBack`.

**Log lines.** Each `text` event with text, and each turn's `result`:
- Control characters are removed with `watch`'s `withoutControls` (`internal/watch/watch.go:282`), exported as `watch.WithoutControls` for this; `watch` keeps using it.
- A `text` equal to the turn's result text is dropped. A `text` that ends with the turn's result text keeps only what comes before it, trimmed; dropped if nothing is left.
- The result becomes one line with `Result: true` and `Text: resultLine(...)`, at the result's time.

`resultLine`: the result's true/false fields, the ones the machine's edges route on first in the order they're written in the graph, then the rest alphabetically, each as `field ✓` or `field ✗`, joined with ` · `; then the `task` field, or else the `summary` field, if it's a string, cut to 80 characters with `…`; then, if `findings` is a non-empty list, `1 finding: ` or `N findings: ` and the first finding: its `issue` field if it's an object with a string `issue`, the finding itself if it's a string, otherwise nothing after the count; cut to 100 characters with `…`. Other fields are left out. An empty result gives `result`.

**The hand-made record** `send-backs.jsonl`, on the demo line, a few seconds per turn: `run started`; `plan_gate` result `{"pending":false,"ready":false}`; `plan_author`; the three reviewers at once; `plan_synthesiser` `{"satisfied":false}`; `plan_author` again; reviewers; `plan_synthesiser` `{"satisfied":true}`; `doer`; `validator` `{"retry":true,"replan":false,"satisfied":false}`; `doer`; `validator` `{"retry":false,"replan":true,"satisfied":false}`; `plan_author`; reviewers; `plan_synthesiser` `{"satisfied":true}`; `doer`; `validator` `{"retry":false,"replan":false,"satisfied":true}`; `planner` `{"complete":true}`; `run finished`. Each turn is `start`, a `text` line, `result`, `usage` and `end`, as the real records have them.

- [ ] **Step 1: Write the failing tests**

```go
func TestDemoEdgeCountsAtNineAndAHalfMinutes(t *testing.T) {
	// counts at 570 by "from->to": start->plan_gate 2, plan_gate->plan_author 1,
	// each plan_author->reviewer 1, each reviewer->plan_synthesiser 1, plan_synthesiser->doer 1,
	// plan_gate->planner 1, planner->doer 3, doer->validator 3, validator->planner 3; every other 0
}

func TestDemoSendBacksAreTheTwoRetries(t *testing.T) {
	// 51 hand-offs; the first at 0.01 is plan_gate; exactly 2 send-backs, both doer,
	// at run times within 0.5 of 1504.99 and 1794.59; the final frame counts validator->doer 2,
	// planner->finish 1, planner->doer 12, plan_synthesiser->doer 1 (15 doer starts in all:
	// 12 from the planner, 1 from the synthesiser, 2 retries)
}

func TestAnUnfinishedRecordEndsUnfinished(t *testing.T) {
	// the demo record's first 201 lines (write them to t.TempDir()): End.State "unfinished",
	// Total within 0.01 of 1187.207, End.Tokens "353.3k"
}

func TestSendBacksAreCountedAndMarked(t *testing.T) {
	// send-backs.jsonl: the final frame counts plan_synthesiser->plan_author 1, validator->doer 1,
	// validator->plan_author 1, planner->finish 1, each reviewer->plan_synthesiser 3;
	// exactly 3 hand-offs are send-backs: plan_author (after not satisfied), doer (retry),
	// plan_author (replan); the last frame has Finished true
}

func TestAFailedTurnCreditsNoEdge(t *testing.T) {
	// release-ride: the doer whose turn failed hands off nothing, and the planner's first start
	// after the resume credits start->planner (count 2 by the end)
}

func TestResultLineIsShortAndReadable(t *testing.T) {
	// validator {"task":"3. Implement collision-safe horizontal movement.","retry":false,
	//   "replan":false,"satisfied":true,"findings":[],"checks":["..."]} ->
	//   "retry ✗ · replan ✗ · satisfied ✓ · 3. Implement collision-safe horizontal movement."
	// plan_gate {"pending":false,"ready":true} -> "pending ✗ · ready ✓"
	// a 300-character summary is cut to 80 characters ending "…"; {} -> "result"
	// the demo's retry at 18:04:45 ->
	//   "retry ✓ · replan ✗ · satisfied ✗ · 11. Connect keyboard controls to the playable terminal session. · 1 finding: Unknown escape sequences can dispatch embedded bytes as game commands."
	// the one at 18:09:35 has "2 findings: Output failures during teardown are not handled reliably."
	// findings [] adds nothing; findings ["plain text"] -> "… · 1 finding: plain text"
}

func TestDemoLogLinesDropTheResultsJSON(t *testing.T) {
	// no Line's Text starts with "{"; the validator's line at the turn ending 17:48:23 reads
	// "I’ll check task 3 against the accepted plan, inspect the movement changes, and run the
	// relevant tests." and is followed by a Result line for the validator
}

func TestLogLinesStripControlCharacters(t *testing.T) {
	// a record whose text is "red \x1b[31mtext\x1b[0m <b>bold</b>" keeps "<b>bold</b>" as text
	// and has no control characters
}
```

- [ ] **Step 2: Run them and see them fail**

Run: `cd factory && go test ./cmd/factory -run 'TestDemoEdge|TestDemoSendBacks|TestAnUnfinished|TestSendBacks|TestAFailedTurn|TestResultLine|TestDemoLogLines|TestLogLines' -count=1`
Expected: FAIL.

- [ ] **Step 3: Implement the crediting, hand-offs, log lines and `resultLine`, and export `watch.WithoutControls`.**

- [ ] **Step 4: Run all the factory's tests**

Run: `cd factory && go test ./... -count=1`
Expected: PASS.

- [ ] **Step 5: Commit** — `git commit -m "Work out which edges a replayed run took, and what each machine said"`

---

### Task 5: The hidden command and its server

**Files:**
- Create: `factory/cmd/factory/visualise.go`
- Test: `factory/cmd/factory/visualise_test.go`
- Create: `factory/internal/replayweb/replayweb.go`, `factory/internal/replayweb/web/index.html` (a placeholder page until Task 6: the title, and a script tag for `replay.js`)
- Create: `factory/internal/replayweb/web/kit/` (vendored brand kit, below)
- Modify: `factory/cmd/factory/main.go:54-60` (the commands map only)

**Interfaces:**
- Consumes: `buildReplay`, `replayNames` (Tasks 3–4), `parseAssemblyLine`, `shownTarget` (`watching.go`), `settings` and `settingsFile` (`runs.go`).
- Produces:
  - `func visualiseCommand(args []string) error`, registered as `"visualise-run"`.
  - `func readRecord(path string) ([]watch.Event, error)`
  - `func replayHandler(doc []byte, files fs.FS) http.Handler`
  - `func runNamesFor(recordPath string) replayNames`
  - `package replayweb`: `func Files() fs.FS` (the `web` folder, embedded with `//go:embed web`).

**The command.** `factory visualise-run [--graph <file>] [--port <n>] <record.jsonl>`, flags before or after the path. Exactly one path, or `usage(...)`. The graph is `--graph`, or `assembly-line.dot` two folders up from the record's folder. Read the record and the graph (no `check()`), build the document, marshal it once. Listen on `127.0.0.1:<port>` (0 picks a free one). Print, to stdout:

```
Replaying run "<run>" on <line> · <target>
Open http://localhost:<port>/
In VS Code, choose "Preview in Editor" when it offers the port, or run "Simple Browser: Show" and paste the address.
Ctrl-C stops it.
```

Then serve until SIGINT or SIGTERM, and shut down cleanly.

**Names.** `runNamesFor`: run = the record's folder's name; line = `Line` from `run.json` beside the record if it reads, else the folder two up's name; target = `shownTarget` of the folder three up from the record's folder (the one holding `.assembly-lines`).

**The handler.** `GET /` serves `index.html`; `GET /run.json` serves the document as `application/json`; any other path is served from `files`, with a 404 for what isn't there. Every response has `Cache-Control: no-store` and a correct `Content-Type`, and **no** `X-Frame-Options` or `Content-Security-Policy`. It doesn't look at `Host` or `Origin`.

**The vendored kit**, from `lean-software-production/brand` at commit `9f95426`: `kit/sketchbook.css` with its first line (the Google Fonts `@import`) replaced by three `@font-face` rules for `Luckiest Guy`, `Patrick Hand` and `Patrick Hand SC` pointing at `fonts/*.ttf` beside it; `kit/sketchbook.js` unchanged; `kit/fonts/luckiest-guy.ttf`, `patrick-hand.ttf`, `patrick-hand-sc.ttf` and their licence files, from `concepts/software-factory/fonts/`; and `kit/README.md` saying where they came from, the commit, and that the fonts are under the SIL Open Font License (Luckiest Guy under Apache 2.0 if its licence file says so: copy what the file says).

- [ ] **Step 1: Write the failing tests**

```go
func TestVisualiseRunIsHidden(t *testing.T) {
	// command([]string{"--help"}): neither stdout nor stderr mentions "visualise";
	// main.go's package comment doesn't either (read the file). Don't call command(nil):
	// with no arguments it starts a run.
}

func TestVisualiseRunNeedsOneRecord(t *testing.T) {
	// command([]string{"visualise-run"}) and with two paths: exit status 2
}

func TestVisualiseRunSaysWhenTheRecordIsMissing(t *testing.T) {
	// a path that doesn't exist: exit 1 and the error names the path
}

func TestReadRecordIgnoresAPartialLastLine(t *testing.T) {
	// two whole lines then `{"time":"2026-` with no newline: two events, no error;
	// a bad line in the middle is an error naming the record
}

func TestNamesComeFromThePathWithoutRunJSON(t *testing.T) {
	// demo record: Run "tetris-007", Line "efficient", Target ending "demo/efficient";
	// release-ride-3 (whose run.json says "ralph-loop"): Line "ralph-loop"
}

func TestHandlerServesThePageAndTheRun(t *testing.T) {
	// httptest: GET / is 200 text/html; GET /run.json is 200 application/json and the document;
	// GET /kit/sketchbook.css is 200 text/css; GET /nope is 404
}

func TestNothingBlocksFraming(t *testing.T) {
	// for /, /run.json, /kit/sketchbook.css, /kit/fonts/patrick-hand.ttf, /nope:
	// no X-Frame-Options, no Content-Security-Policy header; Cache-Control "no-store"
}

func TestAnyHostIsAnswered(t *testing.T) {
	// GET / with Host "fluffy-guide-8080.app.github.dev" and Origin "https://github.dev": 200
}

func TestTheEmbeddedFilesUseNoNetwork(t *testing.T) {
	// walk replayweb.Files(): no .html, .css or .js file contains "http://" or "https://"
	// other than "http://www.w3.org/"; none contains "http-equiv" or "@import"
}
```

- [ ] **Step 2: Run them and see them fail**

Run: `cd factory && go test ./cmd/factory -run 'TestVisualise|TestReadRecord|TestNamesCome|TestHandler|TestNothingBlocks|TestAnyHost|TestTheEmbedded' -count=1`
Expected: FAIL.

- [ ] **Step 3: Vendor the kit**, as above.

- [ ] **Step 4: Implement `replayweb.Files`, `readRecord`, `runNamesFor`, `replayHandler` and `visualiseCommand`, and register the command.**

- [ ] **Step 5: Run all the factory's tests, and try it**

Run: `cd factory && go test ./... -count=1 && go vet ./... && gofmt -l .`
Expected: PASS, no output from `gofmt`.
Run: `go run ./cmd/factory visualise-run ../demo/efficient/.assembly-lines/efficient/runs/tetris-007/record.jsonl --port 8765` in one terminal, `curl -sI http://localhost:8765/` and `curl -s http://localhost:8765/run.json | head -c 300` in another.
Expected: the four printed lines; a 200 with no framing headers; JSON starting `{"run":"tetris-007","line":"efficient"`.

- [ ] **Step 6: Commit** — `git commit -m "Add a hidden visualise-run command that serves a run's replay"`

---

### Task 6: The page draws the line at a moment

**Files:**
- Modify: `factory/internal/replayweb/web/index.html`
- Create: `factory/internal/replayweb/web/replay.css`, `factory/internal/replayweb/web/replay.js`

**Interfaces:**
- Consumes: `GET run.json` (relative), the document from Task 3–4; the kit's `sketchbook.css`, `sketchbook.js` (`#sk-wobble-line`, `#sk-head`, `.sk-bubble`, `.sk-title`, `.sk-hl`).
- Produces, in `replay.js`, for Task 7: `const state = { doc, t, playing, speed }`; `function show(t)` draws the frame at `t` (the last frame with `at <= t`, by binary search); `function fmtElapsed(seconds)` and `function fmtStamp(seconds)`.

What it draws, matching the efficient mockup:
- The header: `<target> · <line> · run "<run>"` on the left, `round <n>` on the right.
- One SVG. Columns by `rank`, rows by `row`, boxes sized to their label and model line. `start` and `finish` dashed pills.
- Edges: between adjacent ranks, a straight line or a gentle S-curve between rows; a forward edge spanning more than one rank, an arc over the top, higher for longer spans; a back edge, an arc underneath, deeper for longer spans. Each label at its edge's middle, on a paper-coloured backing so lines don't run through it. Arrowheads with the kit's `#sk-head`, wobble with `#sk-wobble-line`.
- From the frame: classes `working` and `done` on nodes (✓ after a done machine's name), the model line under each name (blank until known), edges dotted until their count is above 0, a count badge on each edge with a count, blue on back edges.
- The drawing scales to the width of its container (`viewBox` with `width: 100%`), so it fits a 640px pane.
- Machine text and names are set with `textContent`, never `innerHTML`.

`fmtElapsed`: `Ns` under a minute, `Nm Ns` under an hour, else `Nh Nm Ns`, from whole seconds. `fmtStamp`: `m:ss`, or `h:mm:ss` from an hour.

- [ ] **Step 1: Build the page.** Temporarily call `show(570)` after loading, so the demo shows the mockup's moment.

- [ ] **Step 2: Check it by eye**, at 1440 and at 640 wide, with Task 5's server and a headless screenshot (Playwright, as in Task 8). Compare with `2026-10-26-visualise-run-mockup-efficient.html` at 570s: the same ranks and rows; the four back edges underneath in blue; `plan_gate`'s two skips over the top; `doer` yellow; `planner` green with ✓; counts `2` on `start → plan_gate` and `3` on `planner → doer` (blue); `retry`, `replan`, `not satisfied`, `pending` and `complete` dotted. Then the ralph loop (`tetris/release-ride-3`) and `send-backs.jsonl` with `--graph` on the demo line, which must also read cleanly. Save the screenshots in the task's report, not the repo.

- [ ] **Step 3: Run the Go tests** (the embedded-files test still holds)

Run: `cd factory && go test ./... -count=1`
Expected: PASS.

- [ ] **Step 4: Commit** — `git commit -m "Draw the replayed run's line in the brand's sketchbook style"`

---

### Task 7: Playing, the timeline and the bubble

**Files:**
- Modify: `factory/internal/replayweb/web/replay.js`, `replay.css`, `index.html`

**Interfaces:**
- Consumes: `state`, `show(t)`, `fmtElapsed`, `fmtStamp` (Task 6); `doc.handoffs`, `doc.lines`, `doc.breaks`, `doc.end`, `doc.total`.

**The bar** (it wraps on a narrow pane rather than overflowing):
- Play/pause button. The page opens **paused at 0**; the temporary `show(570)` goes.
- Speed buttons `1×`, `10×`, `60×`; `10×` selected at the start. Playing advances `t` by real time × speed, with `requestAnimationFrame`.
- The timeline: filled to `t / total`; a tick for each hand-off, `var(--sk-blue)` for a send-back, else slate; a break mark at each `breaks[].at` with its reason as the tooltip. Click or drag on it (pointer events) to jump there.
- `next hand-off ⏭`: jumps to 1 second before the next hand-off after `t` (not before `t`), or does nothing after the last.
- Text: `<fmtElapsed(t)> of <fmtElapsed(total)> · <frame tokens> tokens`.
- Keyboard: space plays and pauses when the page has focus. Only a bonus.

**The end.** At `t >= total` playing stops. Under the drawing: finished → `The run finished in <fmtElapsed(total)>, using <end.tokens> tokens.` and `finish` lit in forest; stopped or failed → `The run <state> at <fmtElapsed(total)>: <reason>`; unfinished → `The record ends at <fmtElapsed(total)>, before the run finished.` Each break, while `t` is within 5 run-seconds after it, shows `Stopped here, and started again: <reason>` in the same place.

**The bubble**, as in the mockups:
- Click a machine to open its log in a `.sk-bubble`; click it again or ✕ to close; click another to move it. One at a time.
- Heading: the machine's label, then `working` if it's working in the current frame, else `last time`.
- Lines: that machine's `lines` with `at <= t`, newest last, each with its `fmtStamp`; result lines styled as the mockup's `→` line. The newest is highlighted. It scrolls, kept at the bottom as lines arrive while playing.
- Placement: above the machine with the tail pointing down at the machine's centre; if there isn't room above within the frame's window, below with the tail pointing up; kept inside the window; the tail hidden if keeping it inside pulls the tail off the machine. Re-placed on resize, scroll, and when lines are added.

- [ ] **Step 1: Build the bar, the end, and the bubble.**

- [ ] **Step 2: Check by eye**, as in Task 6, at 1440 and 640 wide: paused at 0 on load; play at 60× reaches the stop at 4m 47s and shows the break message; `next hand-off` steps through hand-offs; a click at 26% on the timeline lands near 9m 30s with `doer` working; the bubble on `doer`, on a top reviewer and on `planner` points at its machine; around 25m (1505s) the validator's `retry` shows as a blue tick and a blue count on `retry`; at the end, the "finished" line and `finish` lit.

- [ ] **Step 3: Run the Go tests**

Run: `cd factory && go test ./... -count=1`
Expected: PASS.

- [ ] **Step 4: Commit** — `git commit -m "Play a replayed run, with a timeline, hand-off skipping and machine logs in bubbles"`

---

### Task 8: The browser check, and the build log

**Files:**
- Create: `factory/internal/replayweb/check/check.mjs`, `factory/internal/replayweb/check/frame.html`
- Create: `bin/check-visualise-run`
- Modify: `docs/build-log.md` (a new section at the end)

**Interfaces:**
- Consumes: the built `factory`, the demo record, `send-backs.jsonl`.

**`bin/check-visualise-run`** (bash, `set -euo pipefail`): builds the factory into a temp folder; installs `playwright@1.64.1` into a temp folder with npm (once per run of the script); uses `/usr/bin/chromium` if it exists, otherwise `npx playwright install chromium`; starts `visualise-run` on the demo record with a fixed free port; serves `check/frame.html` from another port with a tiny static server (`python3 -m http.server`), so the page runs **in a frame on another origin**; runs `node check.mjs <frame-url> <replay-url>`; stops both servers on exit. Prints `visualise-run browser check passed` at the end.

**`frame.html`**: an `<iframe>` 640px wide and 720px high whose `src` is the replay address passed in the query string.

**`check.mjs`** asserts, inside the frame:
- it loads, with no console errors and no request to any host but the two local ones;
- one node per machine on the demo line (9 machines, plus `start` and `finish`);
- paused at `0s of 36m 41s`;
- after clicking `60×` and play and waiting 6 seconds, the time has moved past `5m`;
- `next hand-off` moves the time forward;
- clicking `doer` opens a bubble that lies within the frame, with its tail's centre within 20px of the `doer` box's centre horizontally;
- a click at the far right of the timeline shows `The run finished in 36m 41s, using 613.8k tokens.`, `finish` is lit, and the timeline has exactly 2 blue ticks;
- with the demo record's first 201 lines (written by the script to a temp folder beside a copy of the demo graph), the end shows `The record ends at 19m 47s, before the run finished.`;
- with `send-backs.jsonl` (a second `visualise-run` with `--graph`), the timeline has exactly 3 blue ticks, and at the end `finish` is lit and the page says the run finished;
- a record whose text is `<b>bold</b>` (written by the script to a temp folder beside a copy of the demo graph) shows `<b>bold</b>` literally in the bubble.

**The build log section**, "Beyond the homeworks: replaying a run in the browser": what the command does and that it's hidden; why (David's demos); that the logic is in Go and the page only draws; how edges taken are worked out from results with the line's own routing; that tool calls aren't recorded yet and showing them is a follow-up; that it must work in VS Code's Simple Browser in a Codespace, and what that rules out (framing headers, network, `Host` checks); how to run the browser check; and the manual check that remains: David opens the demo run in Simple Browser in a Codespace in the browser, half the screen wide, and plays it to the end.

- [ ] **Step 1: Write the check and the script.**

- [ ] **Step 2: Run it**

Run: `bin/check-visualise-run`
Expected: `visualise-run browser check passed`.

- [ ] **Step 3: Run everything once more**

Run: `cd factory && go test ./... -count=1 && go test -race ./cmd/factory -run 'Visualise|Handler|Replay|Demo' -count=1 && go vet ./... && gofmt -l .` and `bin/release` is **not** run.
Expected: PASS; no stray `factory` or `chromium` processes afterwards (`pgrep -fl 'visualise-run|chromium'` prints nothing).

- [ ] **Step 4: Write the build log section.**

- [ ] **Step 5: Commit** — `git commit -m "Check visualise-run in a browser frame, and add it to the build log"`

# my-factory-reference Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Go factory that passes all eight homeworks of `build-a-software-factory-tutorial`, built as a student would build it, then published as a CalVer-versioned static binary on `my-factory-starter`'s releases. The binary also comes with a `ralph-loop` line that a first-time student can ride.

**Architecture:** One task per homework, each driven by the starter's own skills (`fetch-iteration`, `set-up-factory`, `implement-fast`) against the fetched feature files, which are the test suite. After `homework-008`, three tasks go beyond the homeworks: `watch` presentation and `--version`, the `ralph-loop` line, and the release build. The plan pins only what crosses task boundaries: the project layout, the test commands, the version and progress interfaces, and the release names. The factory's internals are the builder's, as the design says.

**Tech Stack:** Go 1.27 (installed at `/usr/local/go`), godog (latest, v0.16.x) run through `go test`, `pi` with the `openai-codex` login on the build box, `tools/pi-rpc-acp` from homework 7.

**Spec:** `docs/superpowers/specs/2026-10-25-my-factory-reference-design.md`

## Global Constraints

- Work on branch `build-reference-factory` in `lean-software-production/my-factory-reference` (public for now). Push the branch and tags; never push to `main` without David's say-so.
- Never edit `factory/spec/`. If a homework can't be done as written, raise a PR against `lean-software-production/build-a-software-factory-tutorial` (or `my-factory-starter` for a skill bug). Until it merges, carry on with `COURSE_REF=<fix-branch>`. A fix to a homework already done applies from the next fetch, and earlier tags aren't redone.
- Homework commits are the skills' own: `fetch-iteration`'s adoption commit, `Set up the factory project`, and `Implement homework 00N`. Tag each implement commit `homework-00N`. Nothing beyond the spec goes into those commits.
- Default suite: `cd factory && go test ./...`, leaving out `@real-agent`. Real-agent suite: `cd factory && FACTORY_TAGS=@real-agent go test ./steps/ -run TestFeatures -count=1 -timeout 60m`.
- Real runs use `pi` with the `openai-codex` provider (David, 2026-10-25). Run each homework's `@real-agent` examples once, after the default suite passes.
- Machines' logins (`auth.json`) are never committed; the root `.gitignore` already has a safety net.
- `bin/factory` is the student's symlink, and the release binary never claims that path.
- Release tags are `YYYY.MM.DD`, then `YYYY.MM.DD.1`, `.2` for a second or third release that day. Asset names are `factory-<version>-<os>-<arch>` for `linux-amd64`, `linux-arm64` and `darwin-arm64`, plus `SHA256SUMS`.
- Keep a build log in `docs/build-log.md`: one section per homework covering red/green counts, time spent, real-agent results, and each tutorial or starter problem with its PR link. The report back to David is written from it.

## Review Focus

1. **Run from anywhere.** A student runs the release binary by absolute path, or through a symlink, from the repo root. Relative `--seed` and `--target` must resolve from the caller's working directory, not from the binary's location. Test: Task 1, `TestRelativePathsResolveFromCallerCwd`.
2. **Pi missing or signed out.** In a Codespace where Pi isn't signed in, a run must fail within seconds, naming the machine and the harness. It must not hang or print a JSON-RPC trace. Test: Task 7, `TestUnsignedHarnessFailsFast`.
3. **`watch` piped or `NO_COLOR` set.** Output that isn't going to a terminal, or that runs with `NO_COLOR`, must carry no ANSI escapes, so the feature suite and anyone piping to a file see plain lines. Test: Task 9, `TestWatchPlainWhenNotATerminal`.
4. **Odd progress values.** If a result has no `progress`, a `total` of 0, `done` greater than `total`, or non-numeric values, `watch` must not crash. It drops or clamps the bar. Test: Task 9, `TestProgressBarToleratesBadValues`.
5. **An interrupted ride.** If the student closes the terminal or the Codespace sleeps mid-run, starting the same run again must resume its plan, not start over or refuse forever because of a stale daemon. Test: Task 10, a real check (step 4).

---

### Task 1: Homework 001 and the Go project

**Files:**
- Create: `factory/go.mod`: module `github.com/lean-software-production/my-factory-reference/factory`, `go 1.27`.
- Create: `factory/cmd/factory/main.go`: the entry point. `var version = "dev"`, which the release build sets with `-ldflags "-X main.version=<v>"`.
- Create: `factory/factory`: an executable bash launcher. It runs `go build -o "$dir/.build/factory" ./cmd/factory` from its own directory (resolved with `BASH_SOURCE`, without `cd`-ing the caller) and then `exec`s the build with `"$@"`.
- Create: `bin/factory -> ../factory/factory`, a relative symlink.
- Create: `factory/steps/features_test.go`: `TestFeatures` runs godog on `../spec/features`, with tags taken from `FACTORY_TAGS` (default `~@real-agent`).
- Create: `factory/steps/doubles/`: the doubles, each a `main` package. `TestMain` builds them once, into a temp `bin` dir.
- Create: `factory/.gitignore`: `.build/`.
- Modify: `AGENTS.md` (root): a "Checks" section with the two suite commands from Global Constraints.
- Create: `docs/build-log.md`.

**Interfaces:**
- Produces: the test commands above. `steps.TestMain` builds the factory binary once and copies it into each example's workspace; the copy holds only the factory, never `spec/` or `steps/`. Each example gets its own temp dir, which is removed when the example ends.

- [ ] **Step 1:** Commit the spec and this plan: `git add docs && git commit -m "Add the reference factory's design and plan"`.
- [ ] **Step 2:** Say "fetch iteration": follow `.agents/skills/fetch-iteration/SKILL.md`. Expected: `factory/ITERATION` reads `001 WIP`, and the adoption is committed.
- [ ] **Step 3:** Follow `set-up-factory` with Go/godog and the files above. Run `cd factory && go test ./...`. Expected: every step undefined, and no `@real-agent` example runs. Run `bin/factory --seed tetris/spec.md --target /tmp/x`. Expected: the Go binary's "not built yet" message. Commit `Set up the factory project`.
- [ ] **Step 4:** Write `TestRelativePathsResolveFromCallerCwd` in `factory/cmd/factory/paths_test.go`. It builds the binary into a temp dir, runs it from a different cwd with `--seed s.md --target t`, and asserts that the target is created under that cwd. Run it and watch it fail.
- [ ] **Step 5:** Follow `implement-fast` until `go test ./...` passes, including Step 4's test.
- [ ] **Step 6:** Run the real-agent suite. Expected: PASS. Then do the README's "Once your suite passes" run: `bin/factory --seed tetris/spec.md --target tetris/tetris-001 --all`, then `npm --prefix tetris/tetris-001 start` in a pty to confirm it draws a board. Record time and tokens in the build log. Skip the bonus round.
- [ ] **Step 7:** Mark `001 Done` and commit `Implement homework 001` (the skill's step 10). Then `git tag homework-001 && git push origin build-reference-factory homework-001`.

### Tasks 2–8: Homeworks 002 to 008

Each homework is its own task, with the same steps. The table says what's new in each, and which design decisions later tasks rely on.

| Task | Homework | New, and what to watch for | Decisions later tasks rely on |
|---|---|---|---|
| 2 | 002 Checking the work | validator, `machine.feature` | none |
| 3 | 003 Machine configuration | machine folders; `pi` given `PI_CODING_AGENT_DIR`, `--append-system-prompt`, `--no-context-files`; a copied `auth.json` | machine config is `machine.json`: `{"harness": "pi", "model": "<provider>/<model>"}`. Task 10's line uses this format |
| 4 | 004 Assembly line | `assembly-line.dot`, edges route on result fields, attempt limit | parse DOT with a library (gographviz or gonum's `encoding/dot`), not by hand |
| 5 | 005 Runs and targets | lines in `<target>/.assembly-lines/<line>/`, named runs in `runs/<name>/`, run folders committed separately | none |
| 6 | 006 Fan out | parallel branches, a join given only branch results, a failing branch stops the run | none |
| 7 | 007 Watching | daemon; machines as ACP agents via `pi-rpc-acp`; run record in the run folder; `watch`, `stop` | `watch` renders through `internal/watch` (`Render(w io.Writer, ev Event)`), so Task 9 can restyle it without touching the daemon. `Event` carries `Machine string`, `Text string` and `Result map[string]any` (the parsed result line, or nil) |
| 8 | 008 Steering | `say`, `_session/steering` | none |

Task 7 also owns Review Focus 2. Before implementing, write `TestUnsignedHarnessFailsFast` in `factory/internal/machine/harness_test.go`. Point a machine at an empty config dir with a fake `pi-rpc-acp` that exits 1 with "not authenticated" on stderr. Assert that the run fails in under 10s, with an error naming the machine and `pi`.

Steps, for homework N:

- [ ] **Step 1:** Say "fetch iteration". Expected: `factory/ITERATION` reads `00N WIP`, committed.
- [ ] **Step 2:** Run `cd factory && go test ./...`, and record in the build log which examples fail or are undefined. Expected: failures only in what the README calls new. A failing example from an earlier homework whose words didn't change is either a step-drift bug in the tutorial (check with `bin/step-drift` in the tutorial repo) or a regression.
- [ ] **Step 3:** Follow `implement-fast` until `go test ./...` passes.
- [ ] **Step 4:** Run the README's commands through `bin/factory` where they don't call a real agent, then the real-agent suite. Expected: PASS. Record results, time and anything surprising.
- [ ] **Step 5:** Mark Done, commit `Implement homework 00N`, then tag `homework-00N` and push the branch and tag.

### Task 9: `watch` presentation and `--version`

**Files:**
- Modify: `factory/internal/watch/`: option A presentation.
- Modify: `factory/cmd/factory/main.go`: `--version`.
- Test: `factory/internal/watch/render_test.go`, `factory/cmd/factory/version_test.go`.

**Interfaces:**
- Consumes: `watch.Render`, `watch.Event` (Task 7).
- Produces: `progress` in a result is `{"done": <int>, "total": <int>}`; Task 10's planner prompt asks for exactly this. `factory --version` prints `factory <version>\n` and exits 0.

What option A looks like (from the 2026-10-25 mockups):
- **Machine lines.** Each machine's lines are prefixed with its name, padded to the longest machine name, then ` | `. Each machine gets a stable colour from a fixed palette, in order of first appearance.
- **Progress line.** A pinned bottom line reads `▶ <line>  task <done>/<total>  <bar>  <elapsed>`. The bar is 16 cells of `█` and `░`, and elapsed is shown as `6m 40s`. Without a valid `progress`, the line reads `▶ <line>  <elapsed>`.
- **Token line.** Kept as homework 7 has it, dimmed.
- **Plain mode.** When stdout isn't a terminal, or `NO_COLOR` is set, there are no escapes and no pinned lines. Progress then prints as an ordinary line whenever it changes.

- [ ] **Step 1:** Write the failing tests:
  - `TestWatchPlainWhenNotATerminal`: render into a `bytes.Buffer` and assert there's no `\x1b`.
  - `TestProgressBarToleratesBadValues`: a table of `nil`, `{"total":0}`, `{"done":15,"total":12}` and `{"done":"x"}`. Assert no panic, and that the bar is clamped to 16 cells or omitted.
  - `TestProgressBarCounts`: `{"done":5,"total":12}` gives `task 5/12` and 6 filled cells.
  - `TestVersionFlag`: the binary built with `-ldflags "-X main.version=2026.11.03"` prints `factory 2026.11.03`.
- [ ] **Step 2:** Run `cd factory && go test ./internal/watch/ ./cmd/factory/`. Expected: FAIL.
- [ ] **Step 3:** Implement. Detect a terminal with `golang.org/x/term`'s `IsTerminal`.
- [ ] **Step 4:** Run `go test ./...`. Expected: everything passes, including the whole feature suite, since it isn't a terminal.
- [ ] **Step 5:** Commit `Polish watch and add --version`.

### Task 10: The `ralph-loop` line and the first real ride

**Files:**
- Create: `lines/ralph-loop/assembly-line.dot`: `planner → doer`, `doer → planner`, and the planner's edge out on its result's `complete` field ending the run. No validator, no fan out.
- Create: `lines/ralph-loop/machines/planner/AGENTS.md`, `machine.json`, `.gitignore` (ignoring all but those).
- Create: `lines/ralph-loop/machines/doer/AGENTS.md`, `machine.json`, `.gitignore`.
- Create: `bin/ride`: a script for this repo only. It copies `lines/ralph-loop` into `<target>/.assembly-lines/ralph-loop`, copies `~/.pi/agent/auth.json` into each machine, and starts the run. The starter's ride later does the same steps through coach-me.

**Interfaces:**
- Consumes: `machine.json` format (Task 3) and `progress` (Task 9).
- Produces: `lines/ralph-loop/`, which the ride project copies into the starter.

The planner's `AGENTS.md` must have it write the plan in the run's folder and report `{"complete": <bool>, "progress": {"done": <n>, "total": <n>}}` as its result. Both machines' `machine.json` use the `openai-codex` model chosen in step 2.

- [ ] **Step 1:** Write the line. Check that it parses: `bin/factory --run probe --target /tmp/probe --line ralph-loop --seed tetris/spec.md` with a fake `pi-rpc-acp` first on `PATH`, then `factory stop`.
- [ ] **Step 2:** Choose the model. Run the line for real on Tetris with `gpt-6-sol`, then with `gpt-6-luna`, each in a fresh target under `tetris/`, and watch each with `bin/factory watch`. Record wall time, tokens and whether `npm start` draws a playable board in a pty. Pick the fastest one that's playable, and put the comparison in the build log.
- [ ] **Step 3:** Check the bar. While a run is going, `watch` in a terminal shows `task n/N` advancing.
- [ ] **Step 4:** Check Review Focus 5. Start a ride and `kill -9` the daemon partway through. Start the same run again: it resumes the plan from where it was. Then `factory stop`. Record the result.
- [ ] **Step 5:** Commit `Add the ralph-loop line for the first ride`.

### Task 11: Release build

**Files:**
- Create: `bin/release`, which:
  - takes `<version>`;
  - refuses unless the tree is clean and `<version>` matches `^[0-9]{4}\.[0-9]{2}\.[0-9]{2}(\.[0-9]+)?$`;
  - builds the three platforms with `CGO_ENABLED=0 GOOS=… GOARCH=… go build -trimpath -ldflags "-s -w -X main.version=<version>"` into `dist/<version>/`;
  - writes `SHA256SUMS`;
  - tags this repo `<version>`;
  - prints the `gh release create` command without running it.
- Modify: `.gitignore`: `dist/`.

- [ ] **Step 1:** Run `bin/release 2026.13.01`. Expected: refused, because month 13 is invalid. Make the check reject a month or day that's out of range too, not only the pattern. With a dirty tree it's also refused.
- [ ] **Step 2:** Run `bin/release <today>`. Expected: three binaries and `SHA256SUMS`. `file` shows `statically linked` for both Linux binaries. `dist/<v>/factory-<v>-linux-amd64 --version` prints the version.
- [ ] **Step 3:** Run the release binary through `bin/ride` once more on a fresh Tetris target, to check it builds Tetris the way the dev build did. Ask David to play the result before publishing.
- [ ] **Step 4:** Commit `Add the release build`. Push the branch and the version tag.
- [ ] **Step 5:** Show David the printed `gh release create <v> -R lean-software-production/my-factory-starter --title <v> --notes "Built from my-factory-reference@<sha>" dist/<v>/*` command. Publish only when he says so.

The darwin-arm64 binary can't be run on this box. Say so in the release notes, and ask a Mac user to try `--version` and a ride.

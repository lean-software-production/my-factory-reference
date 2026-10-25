# Build log

How the reference factory was built, one homework at a time, the way a
student would build it with the starter's skills. Times are UTC.

Real runs use `pi` with the build box's `openai-codex` login. The box's
`pi` defaults to a provider that isn't signed in, so real runs put a
wrapper first on the `PATH`
(`/home/ensembleworks-agent/work/pi-openai-codex/pi`), which runs
`pi --provider openai-codex --model gpt-6-sol "$@"`. The factory itself
still just runs `pi`. Tokens are summed from pi's session files.

## Homework 001: Basic unvalidated loop

- **Adopted:** iteration `001-basic-unvalidated-loop` from the tutorial's
  `main`.
- **Set up:** Go 1.27 with godog v0.16.0, run through `go test`.
  `bin/factory` links to `factory/factory`, a bash launcher that builds
  `cmd/factory` into `factory/.build/` and runs it.
- **Red:** after setup, 30 examples ran (the 3 `@real-agent` ones left
  out), with all 270 steps undefined. `TestRelativePathsResolveFromCallerCwd`
  failed, because the factory didn't accept `--agent` and didn't build
  anything yet.
- **Green:** 30 of 30 examples and 270 steps pass in about 15s, and so does
  `TestRelativePathsResolveFromCallerCwd`.
- **Mutation checks:** to check the steps can fail, I broke the factory
  on purpose, one way at a time. Each break made examples fail:
  - committing without pathspecs: 4 `target.feature` examples;
  - leaving the seed's path out of the prompt: "What the agent is given";
  - never running `git init`: "A standalone output folder";
  - never committing: 10 examples;
  - never stopping when the result says the plan is complete: examples
    time out.
- **Time:** about 25 minutes from the fetch to a green suite (15:00 to
  15:25), most of it writing the step definitions and the agent double.

### Real-agent examples

`FACTORY_TAGS=@real-agent go test ./steps/ -run TestFeatures -count=1 -timeout 60m`:
**3 of 3 pass** in 6m 30s (15:35:38 to 15:42:11).

- "A Tetris with different details": 3m 09s, four passes. `npm run play`
  draws a board 8 columns wide.
- "The work is given a seed and nothing else": 2m 58s, four passes.
  `npm start` draws a Tetris board.
- "The plan comes from the seed": 23s. One pass writes the plan, then pi
  is asked to judge whether every task comes from the seed. It says yes.
- **Tokens:** 250,743 over the factory's 9 pi sessions: 57,036 input,
  175,616 cache reads and 18,091 output, or $0.33 by pi's reckoning. The
  judge runs without a session, so its tokens aren't counted.

How the steps check a game: they start it in an 80×24 pty and draw its
output on an emulated screen (`hinshun/vt10x`). They look for the tallest
pair of side walls with no wall between them, and judge whether its cells
are one or two characters wide.

The first attempt failed at "`npm run play` in the target starts Tetris".
The game was fine; the step was wrong. vt10x's `Parse` reads only one
buffer, so the screen showed npm's banner and nothing else. Copying the
whole pty stream into the emulator fixed it. I checked the step against
`tetris/tetris-001`, then ran the suite again in full: the run above.

### Once your suite passes: `tetris/tetris-001`

`bin/factory --seed tetris/spec.md --target tetris/tetris-001 --all`,
from 15:29:45 to 15:33:57: **4m 12s**, five passes (the plan, then four
tasks). The agent marked the plan complete in the same pass as the last
task, so the run stopped there, without an extra pass to find nothing
left.

- **Tokens:** 277,045 in all, over 5 pi sessions: 55,462 input, 208,512
  cache reads and 13,071 output. pi reckons the cost at $0.28.
- **Result:** a dependency-free Node game. `npm --prefix
  tetris/tetris-001 start`, run in a pty for 10 seconds, draws a
  10-column board with `[]` cells, the score, lines and controls in
  exactly 24 rows. Pieces fall and lock without crashing.
- **Bonus round:** skipped.

### Problems found

None in the tutorial or the starter. Two things about the build box:

- The box's `pi` defaults to the `opencode` provider, which isn't signed
  in. A student's Codespace defaults to the provider they signed in to,
  so this only affects this box. The wrapper above works around it.
- `pi -p` waits for its standard input to close when it isn't a
  terminal. A factory that runs pi with its standard input still open
  hangs. Go's `os/exec` gives a child `/dev/null` by default, so this
  factory doesn't hang. It's worth knowing if a homework's hints ever show
  a shell loop.

## Homework 002: Checking the work

- **Adopted:** iteration `002-checking-the-work` from the tutorial's
  `main`, at 15:47.
- **Lens:** the README asks the student to pick what the validator looks
  for. The factory's default is "single responsibility", and
  `--lens <words>` changes it. The factory only passes the words on to the
  validator.
- **Red:** with the adopted spec and homework 1's factory, 39 examples ran
  (the 7 `@real-agent` ones left out), with 428 steps: 123 passed, 149
  undefined and 156 skipped. Every example was undefined, because the
  steps of homework 1 said "the agent" where the spec now says the planner,
  the doer and the validator, and `validation.feature` had no steps at all.
  The failures were all in what the README calls new. No step-drift problem
  in the tutorial and no regression.
- **What changed in the factory:** a pass with no plan runs the planner and
  commits the plan. Otherwise the doer does the next task, the validator
  looks at the uncommitted changes, and its findings go back to the doer
  until it is satisfied or `--attempts` (3 by default) run out; then the
  work is committed and the planner marks the task done and says whether the
  plan is complete. New flags: `--planner`, `--doer`, `--validator` (a
  harness for one machine, otherwise `--agent`), `--attempts`, `--lens`.
- **Doubles:** the agent double plays whichever machine its prompt says it
  is ("You are the doer of ..."). A new `spy` double wraps the real `pi`
  and records whether it changed the target, for "a validator reports
  findings, and changes neither the plan nor the work".
- **Green:** 39 of 39 examples and 428 steps pass in about 25s.
- **Mutation checks:** each of these made examples fail: not passing the
  findings to the doer (1 example); treating every verdict as satisfied (5);
  allowing one attempt whatever `--attempts` says (4); leaving the lens
  out of the validator's prompt (1).
- **Time:** about 35 minutes from the fetch to a green suite (15:47 to
  about 16:20), most of it the previous implementer's work.
- **README commands:** the README's command runs a real agent, so through
  `bin/factory` I checked only that the launcher builds and parses the
  arguments: an unrunnable harness reports "could not run the planner
  (no-such-harness)", and a missing seed reports "there is no seed". The
  "Once your suite passes" section and the bonus round were skipped.
  Nothing in homeworks 3 to 8 depends on them (homework 4's README
  mentions that `tetris/tetris-002` would resume its plan, which is
  advice, not a requirement).

### Real-agent examples

`FACTORY_TAGS=@real-agent go test ./steps/ -run TestFeatures -count=1 -timeout 60m`,
with the pi wrapper first on the `PATH`: first run 16:25 to 17:02
(**37m 32s**), **6 of 7 pass**. The failing one was "A validator that looks
at testability". The factory was fine: the validator reported two findings,
that game rules were trapped in a terminal-only loop and that no tests or
test command were added. The step's judge (pi) said the second wasn't
about testability, but about missing tests. A missing test is something
testability asks for, so the step was too strict. I told the judge that a
lack of what the lens calls for counts, and ran the example again: it
passes (4m 20s). The six that passed in the first run were not repeated.

How the steps check the real examples: the "untestable first attempt"
examples commit a plan whose first task asks for an untestable, English-only
piece of code, run a pass with the `spy` around the validator, and judge
the plan and findings with pi.

### Problems found

None in the tutorial or the starter.

### Fix round after review

The `homework-002` tag predates this fix. The review found that the
prompts never said a task isn't done until its subtasks are: the planner
now leaves a task undone while a subtask is open, and the doer marks the
subtasks it has put right. (It also found that my edit of this log had
overwritten homework 1's "Green" line; that's restored.)

Real-agent checks after the fix: "The first task's work is untestable" and
"A finding on the first task" pass (8m 38s together).

## Homework 003: Machine configuration

- **Adopted:** iteration `003-machine-configuration` from the tutorial's
  `main`, at 17:27.
- **Red:** with the adopted spec and homework 2's factory, 43 examples ran
  (the 8 `@real-agent` ones left out): 37 passed and 6 had undefined steps
  (476 steps: 451 passed, 15 undefined, 10 skipped). The 6 were the
  examples new or reworded in `machine.feature`: the validator naming no
  harness, the doer's and the validator's instructions, and the three login
  examples. No step-drift problem in the tutorial and no regression: the
  other 37 kept their words and passed.
- **What changed in the factory:**
  - Each machine has a folder, `factory/machines/<name>/`, holding its
    `AGENTS.md`, a `machine.json` (`{"harness": "pi", "model":
    "<provider>/<model>"}`) and a `.gitignore` that keeps everything but
    those out of Git, its own login and pi's sessions included.
  - The prompts in the factory's code are down to what the factory knows:
    where the seed and plan are, the doer's changes for the validator, the
    findings, and the result it wants. Everything about what a machine is
    and how it works moved to its `AGENTS.md`, homework 2's rules about
    subtasks and the validator's lens included. `--agent`, `--planner`,
    `--doer`, `--validator` and `--lens` are gone: a machine names its own
    harness, and the lens is a line of the validator's `AGENTS.md`
    (`--attempts` stays).
  - A harness is called the way the README calls pi: `PI_CODING_AGENT_DIR`
    set to the machine's folder, then `-p --no-context-files --no-skills
    --model <model> --append-system-prompt <folder>/AGENTS.md <prompt>`.
    Only pi is wired up. A harness other than pi is called the same way (the
    doubles rely on it), and the Claude Code and Codex recipes in the
    README are not built.
  - Before any machine is called, the factory checks that each machine
    can sign in: its `auth.json` has an entry for the model's provider, or
    the environment has that provider's API key (a small table of the
    providers with a single key variable). Otherwise: "the doer cannot sign
    in to ...".
  - The factory finds `machines/` from its own binary (next to it, or in
    the folder above it, which is where the launcher's `.build/` sits), not
    from the working directory.
- **Doubles:** the agent double now behaves like pi at the edge. It reads
  the `AGENTS.md` it is pointed at, records how it signed in (the
  `auth.json` in `PI_CODING_AGENT_DIR`, else an `*_API_KEY` in the
  environment, else it fails like pi), and plays the machine whose result
  field the prompt asks for. The steps copy the real `machines/` folders
  (what Git keeps, never a login) into each example's copy of the factory,
  give every machine the double as its harness and a stand-in login with
  its own name in it. Examples that need an API key move the doer to the
  `anthropic` provider, because `openai-codex` has logins and no key.
- **Green:** 43 of 43 examples and 476 steps pass in about 25s.
- **Mutation checks:** each of these made examples fail: leaving
  `--append-system-prompt` out (2 examples); not setting
  `PI_CODING_AGENT_DIR` (4); pointing every machine at the planner's
  folder (4); skipping the sign-in check (1); a `.gitignore` that doesn't
  ignore the login (1).
- **Unit tests the doubles can't replace:** `TestHarnessIsCalledOnTheMachinesConfigurationOnly`
  (the flags and environment a harness is called with),
  `TestPromptsHoldNoInstructionsOfTheMachines`, and
  `TestInstructionsKeepATaskOpenUntilItsSubtasksAreDone` (homework 2's
  subtask rules, now checked in the `AGENTS.md` files).
  `TestRelativePathsResolveFromCallerCwd` needed machine folders next to
  its binary.
- **Time:** about 10 minutes from the fetch to a green suite (17:27 to
  about 17:37).
- **README commands:** the README's commands run a real agent, so through
  `bin/factory` I checked only the launcher and the checks before any
  machine runs: a missing target and a missing seed are reported, and with
  the doer's `auth.json` moved away the run stops with "the doer cannot
  sign in to openai-codex", before creating the target. The "Once your
  suite passes" section and the bonus round were skipped.

### Machines' logins and models

- Each machine's `auth.json` is a copy of `~/.pi/agent/auth.json`, made by
  hand with `cp` (it also holds an `opencode-go` entry the machines don't
  use). `git check-ignore` prints all three paths and `git add -n
  factory/machines` lists only `.gitignore`, `AGENTS.md` and `machine.json`.
  No login is committed.
- **The wrapper still works with `PI_CODING_AGENT_DIR` set to a machine
  folder.** It only adds `--provider openai-codex --model gpt-6-sol`, which
  pi reads from the command line. What the folder changes is where the login
  comes from, and that is the copied `auth.json`.
- **The machines' `machine.json` name `openai-codex/gpt-6-sol`, and for real
  runs they must name some model that the copied login can use.** The
  machine folder has no `settings.json`, so without `--model` pi falls back
  to its own default provider. Checked by hand: plain pi (no wrapper) in
  an empty folder with the copied login and no model fails with a 401
  from the wrong provider; with `--model openai-codex/gpt-6-sol` it
  answers. So the factory now works with plain `pi` on this box too, and
  the wrapper is no longer needed. With no login and a model, pi fails in
  under a second with "No API key found for openai-codex".

### Real-agent examples

`FACTORY_TAGS=@real-agent go test ./steps/ -run TestFeatures -count=1 -timeout 60m`,
with the pi wrapper first on the `PATH`: 17:39 to 18:04 (**25m 09s**),
**8 of 8 pass** (the 8 of homeworks 1 and 2, plus the new one):

- "Instructions above the target are not the machine's": 3m 48s. The folder
  above the target says "apple", the doer's `AGENTS.md` says "plum", and
  `CODEWORD` says plum.
- "A Tetris with different details": 4m 30s.
- "The work is given a seed and nothing else": 4m 31s.
- "The plan comes from the seed": 18s.
- "A validator that looks at testability": 3m 06s; "...at something else"
  (internationalisation): 2m 34s.
- "The first task's work is untestable": 3m 14s; "A finding on the first
  task": 3m 06s.

The steps for these use the real pi with the copied login in each machine's
folder inside the example's temporary workspace, removed with it.

### Problems found

None in the tutorial or the starter that stopped the build. One to look at:

- **The README's pi recipe leaves the runner's own skills in.** With
  `PI_CODING_AGENT_DIR`, `--no-context-files` and `--append-system-prompt`
  alone, a machine still lists the skills in the runner's
  `~/.agents/skills` (checked: a pi started that way named
  `talking-to-humans` from there). `--no-skills` removes them, and the
  factory passes it. That fits the README's "none of yours", but the
  recipe table doesn't say so. No PR raised: it is for the controller to
  decide whether a README line is worth one.
- Also noticed: the recipe's `--no-session` is left out of the factory's
  call so the sessions stay in the machine's folder (ignored by Git) for
  checking what a machine did.

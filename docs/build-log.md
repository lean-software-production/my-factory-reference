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

## Homework 004: The assembly line

- **Adopted:** iteration `004-assembly-line` from the tutorial's `main`, at
  18:09.
- **Red:** with the adopted spec and homework 3's factory, 49 examples ran
  (the 8 `@real-agent` ones left out): 3 passed and 46 were undefined (515
  steps: 364 passed, 76 undefined, 75 skipped). Every example that runs the
  factory was undefined, because the steps said "the factory runs one pass"
  and the spec now says "the factory runs"; `assembly-line.feature` was new.
  The failures were all in what the README calls new. No step-drift problem
  in the tutorial and no regression.
- **What changed in the factory:**
  - The line is `factory/assembly-lines/careful/`: `assembly-line.dot`, and
    the three machines' folders (moved there, unchanged, from
    `factory/machines/`) in `machines/` beside it. The factory has one line,
    the only folder in `assembly-lines/`, found from the binary like
    `machines/` was.
  - The graph is read with `gographviz` (not by hand), before any machine is
    called. The factory refuses a line that names a machine with no folder
    ("no machine called ..."), that has an edge to `finish` from any machine
    but the planner, or on which `finish` can't be reached from some node
    ("finish cannot be reached from validator").
  - The pass is gone. The factory goes along the line from `start`, calls the
    machine it comes to, and takes the first edge leaving it that the result
    takes: an unlabelled edge always, `field` when the field is true,
    `not field` when it is false. A result without the field, or with a
    field that is not true or false, stops the run, naming the machine and
    the field.
  - What the line doesn't say is the factory's. The work is committed, with
    the doer's `task` as its message, when the line goes from a machine back
    to the planner; the planner's changes to the plan are committed after it
    runs. The attempt limit (`--attempts`, 3) counts how often each machine
    is called for a task, and a task is over when the planner is called; one
    more than the limit stops the run with "a task hit its limit".
    `--all` is gone.
  - Prompts ask each machine for the fields its edges route on (so
    relabelled edges change the prompt), plus the doer's `task` and the
    validator's `findings`. A machine on the line the factory has no prompt
    for gets the seed, the plan and its routed fields.
- **Doubles:** the agent double now plays the machine whose folder it is
  pointed at (`PI_CODING_AGENT_DIR`), rather than guessing from the field the
  prompt asks for, because the field can now be named by the line. New steps
  edit the example's `assembly-line.dot` as text. "The factory reads the
  assembly line" runs the factory on a seed with the doubles; "it refuses it"
  means a failed run in which no agent was called.
- **Green:** 49 of 49 examples and 515 steps pass in about 38s.
- **Mutation checks:** each of these made examples fail: not checking that a
  machine has a folder (1 example, "A misspelt validator"); not checking that
  `finish` can be reached (1, "Satisfied work has nowhere to go"); taking
  every edge whatever the label (17); ignoring `--attempts` (2); resetting
  the counts for a task never (1, "The work is wrong first time"); never
  committing when the line returns to the planner (the suite hangs: the
  planner never marks a task done, so the run never ends, and the 10-minute
  `go test` limit fails it).
- **Unit tests the doubles can't replace:** `TestLineIsReadWhateverIsQuoted`
  and `TestLineThatIsNotAWellFormedDigraphIsRefused` (the library's reading
  of quoted names and labels, and bad graphs).
  `TestRelativePathsResolveFromCallerCwd` needed an assembly line next to its
  binary.
- **Time:** about 30 minutes from the fetch to a green suite (18:09 to about
  18:40), most of it rewriting the run loop and the steps together rather
  than one example at a time; the mutation checks took another 15 minutes.
- **README commands:** `bin/factory --seed tetris/spec.md --target
  tetris/tetris-004` runs a real agent, so through `bin/factory` I checked
  only the launcher and the checks before any machine runs: a missing target
  and a missing seed are reported; in a copy of the factory, a line with the
  edge from the validator to the planner taken out is refused ("finish cannot
  be reached from doer; ... from validator"), and so is one naming
  `validater`, in both cases before the target is created. The "Once your
  suite passes" section and the bonus round were skipped.

### Real-agent examples

`FACTORY_TAGS=@real-agent go test ./steps/ -run TestFeatures -count=1 -timeout 60m`,
with the pi wrapper first on the `PATH`: 18:51 to 19:34 (**43m 19s**),
**8 of 8 pass** (`go test` ran without `-v`, so there are no times for the
single examples). Every example now runs the whole line, validator included,
until the planner says the plan is complete, where homeworks 2 and 3 ran
one pass. That is why the run took longer than homework 3's 25 minutes.

- To keep the two validator examples to one task each, the step that sets
  the validator's lens and "the doer's first attempt at a task is
  untestable" now commit a plan of just the one flawed task (before, it had a
  second task, "Show the score", which a pass never reached).
- Tokens weren't summed: the sessions are in the examples' temporary
  folders, removed with them.

### Problems found

None in the tutorial or the starter. One thing to note:

- "The factory reads the assembly line" has no seed in its Background, and
  the spec doesn't say how a factory is to read a line without being run. I
  read it as "run the factory on a seed with do-nothing machines", and a
  refused line as a failed run in which no machine was called.

### Fix round after review

The `homework-004` tag predates this fix. The review found the factory
recognised the planner, the doer and the validator by name (the finish
check, the commit trigger and attempt reset, the findings going back, the
prompts), so a renamed or added machine got the wrong treatment. Now the
planner is the one machine with an edge to finish, whatever it is called
(a line where two machines have one is refused); a machine the planner sends
work to is asked for `task` (it names the commit); a non-planner machine with
a "not" edge is asked for `findings`; and every machine is given the same
context (seed, plan, the previous machine's result, the uncommitted changes).
Role-specific instructions stay in each `AGENTS.md`. New tests: unit tests for
the roles read from the graph and for the single edge to finish, and two runs
of a line with the machines renamed `lead`, `builder` and `checker`
(`steps/renamed_machines_test.go`). The agent double now decides which
machine it plays from the `# Planner`/`# Doer`/`# Validator` heading of the
AGENTS.md it is given, as a real agent would, not from its folder's name.
Default suite 49/49 plus the new tests; real-agent suite 8 of 8 again, 19:42
to 20:14 (**31m 48s**).

## Homework 005: Runs and targets

- **Adopted:** iteration `005-runs-and-targets` from the tutorial's `main`,
  at 20:16.
- **Red:** with the adopted spec and homework 4's factory, 55 examples ran
  (the 8 `@real-agent` ones left out): all 55 were undefined (630 steps:
  55 passed, 246 undefined, 329 skipped). Every example's Background changed
  (a target that holds a line, and a named run on it), and every "the
  factory runs" became "the factory runs the "tetris" run". The failures were
  all in what the README calls new. No step-drift problem in the tutorial and
  no regression.
- **What changed in the factory:**
  - The lines moved into the target: `tetris/tetris-005/.assembly-lines/`
    holds `careful` (homework 4's folder, moved with `mv` so its machines'
    logins came along, and with a line added to each `AGENTS.md` about the
    run's folder) and a new `quick` line, the same without the validator.
    The factory no longer has `assembly-lines/` and no longer looks beside
    its binary for anything.
  - A run is `--run <name> --target <folder>`, and the first time also
    `--line <line> --seed <file>`. Its folder is
    `<target>/.assembly-lines/<line>/runs/<name>/`, holding `plan.md` and a
    `run.json` that remembers the line and the seed (the seed as a path from
    the target, so a clone elsewhere finds it). After that the name and the
    target are enough. A run is found by looking through the target's lines
    for its name, so a name is unique in the target; naming it with another
    line or seed is refused ("the "tetris" run already has a line").
    `--attempts` stays a flag of the invocation, not a setting of the run.
  - The target is always named. A missing target, or a run that hasn't been
    started and isn't given a line and a seed, is reported before anything
    is read or written.
  - The run's folder is committed apart from the work: when the run starts
    ("Start the tetris run"), after each planner call, and once more at the end
    of an invocation however it ends. The work commit leaves out
    `.assembly-lines/`, and so does the list of uncommitted changes machines
    are given. The plan is no longer committed with the work, and no longer
    in `.factory/`.
  - Machines are told where the run's folder is, as one more line of the
    same briefing every machine gets.
- **Doubles:** the agent double finds the plan from the prompt ("The plan is
  ..."), and keeps a note of each call in the run's folder, as a machine told
  to keep what it makes there would. That is what gives "the run's folder has
  no uncommitted changes" something to check when a task gives up. The spy
  now watches the work and the plan, not the rest of the run's folder.
- **Steps:** a world keeps the runs an example names and the targets it
  makes. The lines in a target are copies of `tetris/tetris-005`'s (only what
  Git keeps, with the double as every machine's harness and a stand-in
  login). "The run has been started" runs the factory for real, then forgets
  the machines' calls so that the example asks about what happens next.
  "The target is outside any Git repository" moves the target, lines and
  all, out of the test repository.
- **Green:** 55 of 55 examples and 630 steps pass in about 50s.
- **Mutation checks:** each of these made examples fail: committing the
  work with `.assembly-lines/` (1 example, "Three tasks"); no commit of the
  run's folder at the end of an invocation (1, "A task that gives up"); letting
  a run's line be changed (1); always taking the `careful` line whatever the
  run's (2, "Two lines, one target" and "Two lines, two doers"); leaving the run's
  folder out of the briefing (nearly every example, because the agent double then can't keep its
  note). Two mutations changed nothing in the suite: not committing the plan
  after each planner call (the final commit covers it) and finding a started
  run on one line only (a started run is only resumed on `careful` in the
  examples). The second is covered by a unit test.
- **Unit tests the doubles can't replace:** `runs_test.go` (a run is found on
  whichever line it is on, its seed is kept as a path from the target, a name
  is unique in its target, a run needs a target and a name).
  `TestRelativePathsResolveFromCallerCwd` now builds the line in the caller's
  target, with nothing beside the binary, and checks the run's folder is
  there.
- **Time:** about 20 minutes from the fetch to a green suite (20:16 to about
  20:36), most of it rewriting the world of the steps; the mutation checks
  and the README commands took another 20.
- **README commands:** `bin/factory --run tetris --target tetris/tetris-005
  --line careful --seed tetris/spec.md` runs a real agent, so through
  `bin/factory` I checked only the launcher and the checks before any machine
  runs: a missing target, run, seed, line or line the target lacks is
  reported. With the agent double as every machine's harness, in a copy of
  the target in a scratch repository, I ran the README's commands: the
  `tetris` run on `careful` (planner, doer and validator, with the run's
  folder committed apart from the work), the same run again by name, the same
  run given `--line quick` (refused), and `snake` on a copy of `quick` without
  its runs, in a second target. The "Once your suite passes" section and the
  bonus round were skipped.

### Real-agent examples

`FACTORY_TAGS=@real-agent go test ./steps/ -run TestFeatures -count=1 -timeout 60m`,
with the pi wrapper first on the `PATH`: 20:37 to 21:08 (**30m 29s**),
**7 of 8 pass**:

- "Instructions above the target are not the machine's": 4m 07s;
  "A Tetris with different details": 3m 51s; "The work is given a seed and
  nothing else": 4m 20s; "The plan comes from the seed": 4m 22s; "A validator
  that looks at testability": 3m 12s; "...at something else": 3m 05s; "The
  first task's work is untestable": 2m 58s.
- "A finding on the first task" failed: the validator found nothing wrong,
  because it never saw the flawed task. The example commits a plan of that one
  task, and this time the planner rewrote the plan as the whole of the seed
  (three tasks) on its first call, so the run built a clean Tetris. The factory
  was fine. Run again by itself, the example passes (3m 23s), and so did the
  same set-up by hand in a scratch repository (the planner kept the plan, the
  validator found the missing tests, the doer put them right). It is the
  planner being free to do that: its `AGENTS.md` says what to do with no plan
  and after the doer's work is committed, but nothing for a plan that exists
  with nothing committed yet. I left the prompt alone for the review to decide.
  The logs are in `.superpowers/sdd/2026-10-25-my-factory-reference/logs/`
  (`005-real-agent.log`, `005-real-finding-rerun.log`).
- Tokens weren't summed: the sessions are in the examples' temporary folders.

### Problems found

None in the tutorial or the starter. One thing to note:

- The README's `quick` line is "the same line without the validator", and its
  layout shows `planner` and `doer` only under `quick/machines/`. Its
  doer's `AGENTS.md` loses the section about the validator's findings; that is
  my reading of "configured differently".

### Fix round after review

The `homework-005` tag predates this fix. The review found two things.
The steps copied a line from `tetris/tetris-005` with any `runs/` it held,
so after a real run on that target every example would start with the
"tetris" run already started: copies now leave `runs/` out
(`TestACopyOfALineHoldsNoRuns`). And the planner's `AGENTS.md`, on both
lines, said nothing about a plan that already exists, which is how the
flake above came about: it now says to keep an existing plan's tasks as they
are, in order, and only mark done the tasks whose work is committed
(`TestPlannersKeepAnExistingPlansTasks`). Default suite 55/55 plus the unit
tests; real-agent suite 8 of 8 again, 21:20 to 21:52 (**31m 13s**).

## Homework 006: Fan out

- **Adopted:** iteration `006-fan-out` from the tutorial's `main`, at 21:54.
- **Red:** with the adopted spec and homework 5's factory, 70 examples ran
  (the 9 `@real-agent` ones left out): 55 passed and 15 were undefined (821
  steps: 687 passed, 77 undefined, 57 skipped). The 15 are the 12 examples
  of the new `fan-out.feature` and the 3 new ones in `assembly-line.feature`.
  The only other change to the features is a sentence in `machine.feature`'s
  rule text. The failures were all in what the README calls new. No
  step-drift problem in the tutorial and no regression.
- **The three big brains:** a third line in the first target,
  `tetris/tetris-005/.assembly-lines/three-big-brains/`, with the README's
  graph. Its planner and doer are the `careful` line's (the doer's heading
  about findings now names the synthesiser). `reviewer_1` to `reviewer_3`
  have the same `AGENTS.md` (lens: testability) and differ only in their
  model: `openai-codex/gpt-6-sol`, `gpt-6-luna` and `gpt-6-astra`. The
  course means three providers' models, and a student with three
  providers' logins would use three providers. Real runs on this box use only
  the `openai-codex` login (David's decision), and the features check only
  "different models" and "the same prompt". The `synthesiser` (gpt-6-sol)
  reads the three reports and decides. Every machine has its own copy of the
  login, which Git ignores.
- **What changed in the factory:**
  - A machine with more than one edge out and no labels fans out. Before any
    machine runs, the line is refused ("the branches from doer do not meet")
    unless every branch has one unlabelled edge out, and all those edges lead
    to the same machine.
  - The branches run at the same time, one goroutine each, and the factory
    waits for all of them. Each branch gets the same briefing as any machine
    (seed, plan, run's folder, the doer's result, the uncommitted changes
    listed once, before any branch starts) and is asked for its own result.
    Each branch counts towards the attempt limit like any machine.
  - The machine where the branches meet is told only what each branch
    answered, under its name, and is asked for the fields its edges route on.
    The machine after it is told only the meeting machine's result, so the
    doer gets the synthesiser's findings and not the reviews.
  - A branch that fails (cannot be run, crashes, or answers with no result)
    cancels the others' context. The factory waits for them to die, names the
    branch ("a branch from doer failed, so the factory stopped the others:
    could not read the reviewer_2's result"), runs nothing after them and
    commits no work. The run's folder is still committed at the end, as
    before.
  - Every machine now runs in a process group of its own. The group is killed
    when the call's context is cancelled, and again once the machine has
    answered, so nothing a machine started outlives its call. Its answer goes
    to a temporary file, not a pipe, so a process it leaves behind can't hold
    the call open. Because a machine in its own group no longer gets the
    terminal's Ctrl-C, the factory now handles SIGINT, SIGTERM and SIGHUP
    itself and stops the machines running before it exits. Homework 7 makes
    stopping a run a feature; this is only what concurrency needed.
  - The factory now prints each result with its machine's name,
    `reviewer_2: {...}`, because three branches' results arrive in any order.
    They are printed one at a time.
- **Doubles:** the agent double can now run as several processes at once.
  Calls take turns, under a `flock` on the double's folder, at numbering
  themselves and writing `calls.log`, the prompts and the sign-ins.
  `calls.log` also records the machine's folder, so `reviewer_2` can be told
  apart from the other reviewers (all three play "# Reviewer"). It plays a
  reviewer (satisfied unless told otherwise, or "report <n>" in the order
  they report) and the synthesiser (like the validator). The synthesiser is
  the one machine the double doesn't expect to be told the plan or the run's
  folder. "No reviewer finishes until all three have begun" makes each
  reviewer wait until three reviewer calls have been logged for its attempt.
  It gives up if the factory that called it goes away, so a factory that runs
  the branches one by one ends at the suite's 30s limit, not never. Each call
  notes its process in `pids.log`. "The factory has stopped" now also checks
  that none of those processes is still alive.
- **Steps:** "the line has the three big brains" copies the four machines
  from the student's `three-big-brains` line (only what Git keeps, no runs),
  with the double as their harness and a stand-in login. "The three big
  brains has replaced the validator" does that, removes the validator's
  folder and edits the graph as `STEPS.md` says. "It was not given ..." now
  checks every call to the machine the previous step was about, not only its
  latest. "Could not read/run reviewer_2" accept the factory's "the
  reviewer_2". `FACTORY_FEATURES` (feature files or `file:line`) runs part of
  the suite, used to work one example at a time.
- **Green:** 70 of 70 examples and 821 steps pass in about 65s, at 22:37.
- **Concurrency checks:** `go test -race ./cmd/factory/` passes, including
  the new `fanout_test.go`. A branch that answers in prose while the other
  two are at work (each with a child process), or that can't be run, stops
  the fan out within the 10s limit. Every process and child is gone
  afterwards, and the goroutine count is back where it was. Cancelling a
  running machine kills it and its child. A machine that leaves a background
  process running answers at once, and the process is killed. The fan-out
  and assembly-line features also pass with the factory and doubles built
  with `-race` (`GOFLAGS=-race`; log `logs/006-race-fan-out.log`, 111s).
  The whole suite race-built takes over `go test`'s 10 minutes.
- **Mutation checks:** each of these made tests fail: running the branches
  one after another ("Reviewers that wait for each other", at the 30s limit);
  not checking that branches meet (the 2 refusal examples); giving the
  meeting machine the ordinary briefing ("What the synthesiser is given",
  "The reports disagree"); not cancelling the others when a branch fails (the
  unit test's 10s limit); not giving machines a process group of their own
  (the three process unit tests); not killing the group after a machine
  answers (`TestAMachineLeavesNothingRunning`). One mutation changed nothing:
  listing the uncommitted changes separately for each branch rather than once
  for all. The doubles' reviewers change nothing, so their lists are the same
  either way.
- **Unit tests the doubles can't replace:** `fanout_test.go` above;
  `TestAFanOutIsReadFromTheLine`;
  `TestTheThreeBigBrainsAreAlikeButForTheirModels` (the student's reviewers
  share one `AGENTS.md` and name three models).
- **Time:** about 45 minutes from the fetch to a green suite (21:54 to
  22:37), including the unit tests and mutation checks. The examples went one
  at a time with `FACTORY_FEATURES`, but most of the fan-out examples passed
  as soon as their steps were defined: the fan out was written concurrent from
  the first example (the controller's decision that fan out means
  concurrency), so "Reviewers that wait for each other" was never red. The
  sequential mutation above shows it would have been.
- **README commands:** the README has none to run. Through `bin/factory`,
  from the repository root, I ran the `three-big-brains` line with the agent
  double as `pi`, on a copy of the line in a scratch repository: two tasks,
  the synthesiser rejecting the first attempt. Every attempt's three reviews
  arrived in a different order (reviewer_3, 2, 1, then 2, 1, 3, then
  1, 3, 2), and the run committed `alpha` and `beta` and finished. The
  README's "Once your suite passes" section and bonus round don't exist this
  time.

### Real-agent examples

`FACTORY_TAGS=@real-agent go test ./steps/ -run TestFeatures -count=1 -timeout 75m -v`,
with the pi wrapper first on the `PATH`, ran from 22:38 to 23:14 (**35m 16s**).
I gave it 75m rather than 60m because a fan out makes more agent calls; it
didn't need it. **8 of 9 passed:**

- "Reviewers that look at testability" (new): 4m 58s. The real line went
  from the planner (which kept the one flawed task) to the doer, then to
  three reviewers on gpt-6-sol, gpt-6-luna and gpt-6-astra at the same time,
  then to the synthesiser. That happened three times. The first time all
  three reviewers found the work untestable, each in its own words
  (reviewer_3 gave its findings as objects with a `file`, not as strings),
  and the synthesiser merged them into one finding. The second time two
  reviewers were satisfied and one wasn't, and the synthesiser sided with the
  one, on a real point: the display's 24-row limit was untested. The third
  time all were satisfied, and the planner finished the plan. The judge
  agreed that the reviewers' findings were about testability.
- The others: "Instructions above the target are not the machine's" 4m 42s;
  "The assembly line is given a seed and nothing else" 4m 01s; "The plan
  comes from the seed" 5m 08s; "A validator that looks at testability"
  3m 04s; "...at something else" 2m 56s; "The first task's work is
  untestable" 2m 44s; "A finding on the first task" 3m 16s.
- "A Tetris with different details" (homework 1's example) failed at "its
  board is 8 columns wide". The game was right: it was titled "TETRIS (8
  columns)" and drew each cell as two characters, `@ ` (a glyph, then a
  space), 16 characters inside the walls. The suite's board reader decides
  cells are one character wide if any block run inside the board has an odd
  length, and a lone `@` followed by its blank half is such a run. So it
  counted 16 columns. This is a limit of a homework 1 step, not of the
  factory or of fan out, and I left it for the review rather than tune the
  heuristic to one board. Run again by itself, the example passed (3m 25s).
  Logs: `logs/006-real-agent.log` and
  `logs/006-real-different-details-rerun.log`.
- No machine process was left running after the suite.
- Tokens weren't summed: the sessions are in the examples' temporary folders.

### Problems found

None in the tutorial or the starter. Things to note:

- The spec leaves open where the three big brains' machines come from. I
  read "the line has the three big brains" (STEPS.md: four machines
  "configured alike but for their models") as copies of the student's own,
  kept on a `three-big-brains` line in the first target.
- "Given only their results and nothing else" I took literally: the
  synthesiser isn't told the seed, the plan or the run's folder.
- The words "could not read reviewer_2's result" and "could not run
  reviewer_2" drop the "the" that the validator's and doer's versions have.
  The factory says "the reviewer_2" as it says "the validator", and those
  two steps accept either.

### Fix round after review

The `homework-006` tag predates this fix. The review found two things.

- **pi's tool commands escaped the kill.** pi's bash tool starts each
  command detached, in a session of its own, so killing pi's process group
  never reached it. pi kills those commands itself on SIGTERM or SIGHUP, but
  the factory only ever sent SIGKILL, so a stopped reviewer's running
  `npm test` was orphaned. Now a machine asked to stop gets SIGTERM to its
  group and 3 seconds to clean up. Then Go kills the harness, and whatever
  is still in its group gets SIGTERM, 3 more seconds, then SIGKILL. A
  machine that answered with nothing left in its group costs no wait. New
  tests: a pi-like harness (a `setsid` child that it kills on TERM, with
  SIGKILL as pi does) is stopped with its child gone, both as a failed
  branch's sibling and when the factory is stopped. A harness deaf to
  SIGTERM is killed after the grace period. Mutations: SIGKILL instead of
  SIGTERM, and no final SIGKILL, each failed those tests. One limit
  remains: a server started by a tool command that has already returned is
  untracked by pi and in a session of its own. Neither pi nor the factory
  can reach it, as before this homework.
- **`nohup` and background runs stopped on a hangup or Ctrl-C.** Asking Go
  to catch a signal re-enables one the factory was started with ignored. Now
  the factory catches only the signals of SIGINT, SIGTERM and SIGHUP that it
  wasn't started ignoring. With all three ignored it catches none, because
  asking for no signal in particular would catch every signal.
  `TestAHangupStopsARunUnlessHangupsAreIgnored` starts the factory with
  SIGHUP ignored and sends it one: the planner carries on and the run ends
  well. Started normally, the same SIGHUP stops the factory and its planner.
  Mutation: always catching the signals failed the first case.

How Ctrl-C behaves now, compared with homework 5:

- **Homework 5:** pi was in the factory's process group, so Ctrl-C reached
  both. The factory died at once, without committing the run's folder.
  pi died of the SIGINT, which it doesn't handle, so its detached tool
  commands were orphaned.
- **Now:** machines are in groups of their own, so Ctrl-C (or a closed
  terminal, or SIGTERM) reaches only the factory. The factory stops each
  machine running as above (pi kills its tool commands), commits the run's
  folder ("Keep what the tetris run made"), and exits 1 with "the doer (pi)
  was stopped". Started with `nohup` or as a background job with the
  signals ignored, the factory behaves as it did in homework 5: the run
  carries on, and its machines, which inherit the ignored signals, do too.

While writing the tests, two flaky passes turned up. Both were in the tests
themselves, not the factory. One was a check-then-read race on `/proc`: a
zombie reaped between the two calls counted as alive. The other was the
`sh` harness losing a TERM that reached its forked child before exec.
After fixing them, 48 race-built runs of `cmd/factory`, 8 at a time, all
passed, with nothing left running. Default suite 70/70; `go test -race
./cmd/factory` passes.

The whole suite race-built, `cd factory && GOFLAGS=-race go test ./...
-count=1 -timeout 40m`, passed with no data race: `cmd/factory` in 11.6s and
`steps` in 648s, 23:39 to 23:50 (`logs/006-fix1-race-full.log`).
"Reviewers that look at testability" on real pi passed again in 4m 58s
(`logs/006-fix1-real-fan-out.log`). It went three attempts, as before.
The new kill path only changes how a machine is stopped, not how it
finishes, so the other real-agent examples were not rerun.

## Homework 007: Watching

- **Adopted:** iteration `007-watching` from the tutorial's `main`, at 23:52.
- **Red:** with the adopted spec and homework 6's factory, 86 examples ran
  (the 9 `@real-agent` ones left out): 70 passed and 16 were undefined (1031
  steps: 964 passed, 54 undefined, 13 skipped). The 16 are the new examples
  of `monitoring.feature` (3), `observability.feature` (6) and
  `orchestration.feature` (7). The other changes to the features are rule
  text in `machine.feature` and `orchestration.feature`. The failures were
  all in what the README calls new. No step-drift problem in the tutorial
  and no regression.
- **The target:** the README's commands build `tetris/tetris-007`, so its
  three lines (`careful`, `quick`, `three-big-brains`) are copies of
  `tetris-005`'s, made with `cp -r` so the logins came along (Git ignores
  them). Each machine's `machine.json` now names its ACP agent,
  `{"harness": "pi-rpc-acp", "model": ...}`. `tetris-005` is left as
  homework 6 had it. The suite copies its lines from `tetris-007` now.
- **What changed in the factory:**
  - **Machines are ACP agents** (`internal/machine`). The factory starts
    the machine's agent in the target, with `PI_CODING_AGENT_DIR` set to its
    folder and the arguments `pi-rpc-acp` passes on to pi:
    `--no-context-files --no-skills --model <model> --append-system-prompt
    <folder>/AGENTS.md`. `--no-skills` is still needed (homework 3), and the
    bridge passes it on: a pi started this way named the codeword in its
    `AGENTS.md` and said it had no skills. The factory sends `initialize`,
    `session/new` (`cwd` the target) and `session/prompt`. It reads the
    `agent_message_chunk` updates a line at a time, the last JSON line as
    the result, and the `usage` on the response. A machine naming no
    harness gets `pi-rpc-acp`. Harnesses are found on the caller's `PATH`,
    not the daemon's.
  - **Stopping a machine** works as in homework 6: SIGTERM to its process
    group, 3s, SIGKILL. An agent that has answered is told it is done by the
    end of its input (the bridge then kills pi), and is stopped if it is
    still there after 3s. An agent that exits before answering fails the
    call at once, with what it wrote on its standard error. So does one that
    ends its turn having generated nothing (see Problems found).
  - **The daemon** (`cmd/factory/daemon.go`, `client.go`). `bin/factory
    --run ...` resolves its paths from the caller's working directory, sends
    them and the caller's environment to the daemon, and returns with
    `started run tetris` once the run is checked, its folder is committed
    and its record has begun. The daemon is started by the first command
    that needs it, in a session of its own, and listens on a Unix socket in
    a folder in the system's temporary folder named for the factory's
    program (one daemon per factory). It holds a lock while it runs, so a
    socket left by one that crashed is known to be stale. It runs one run at
    a time: another start is refused with `refused: run tetris is running`.
    `bin/factory stop tetris --target ...` stops the run and returns once it
    has stopped and its folder is committed; `bin/factory stop` does that
    and then stops the daemon, and returns once it has gone. A stop is
    SIGTERM to every machine running, so the attempt is abandoned: no work
    is committed and the planner is not called. What the machines wrote is
    left in the target. The daemon catches only the signals of SIGINT,
    SIGTERM and SIGHUP it wasn't started ignoring, as homework 6's fix did,
    and a daemon started under `nohup` inherits the ignored SIGHUP.
    `bin/factory status` says whether the factory is running, and which run.
    It isn't in the README; the steps use it to check "the factory is
    running" and "the run is running".
  - **The record** is `record.jsonl` in the run's folder, beside its plan:
    an event a line, `watch.Event` as JSON. The events are the run starting
    and ending (finished, stopped or failed, with why), each machine
    starting its turn (with its prompt), each line it generates, its result,
    what its turn spent (tokens in and out, provider and model), and its turn
    ending. It is committed with the run's folder: after each planner call,
    and at the end of every invocation, however it ended. A run started again
    adds to the same record.
  - **`bin/factory watch tetris --target ...`** reads the record from the
    start, then follows it, until the daemon is no longer running the run.
    It renders through `internal/watch` (`watch.New(w, line)`,
    `(*Watcher).Render(ev)`), plainly: `doer       | <line>`, a `running`
    line when the machines running change, a `tokens` line after each turn
    (the machine's totals, each provider's, and the run's), and how the run
    ended. It exits 0 only if the run finished. Watching a finished run
    needs no daemon.
  - The factory no longer prints results as `machine: {json}`; watching
    shows them.
- **Doubles:** the agent double is now an ACP agent. It reads its prompt
  from `session/prompt`, sends what it says as `agent_message_chunk`s, and
  reports usage (a token in per byte it was given, a token out per byte it
  said). It keeps what each call said (`said/<n>.txt`) and spent
  (`usage.log`). A file `hold-doer` or `hold-reviewer` in its folder holds
  that machine part-way through its job, noted in `holds.log`, until the
  file goes, or the factory closes its input. The doer can say lines before
  and after the hold, and a reviewer says `satisfied` before its result. The
  fake pi on the suite's `PATH` is now called `pi-rpc-acp`. The spy sits
  between the factory and the real `pi-rpc-acp`, passing ACP through and
  noting the target before each prompt and after its answer.
- **Steps:** "the factory runs the run" starts it, then runs `watch` to the
  end, and what the factory said is both commands' output. The steps that
  leave a run running hold the doer or reviewers and wait until the double
  says they are held. "I watch" and "I am watching" start `watch` in a
  process of their own and wait for what they expect to appear. The token
  checks compare what `watch` shows with what the doubles reported, to the
  precision shown (12.4k is 12,350 to 12,450). "I read the run's record"
  reads `record.jsonl`. Every example ends by stopping the factory, and fails
  if a daemon or a double is still running after that.
- **Green:** 86 of 86 examples and 1031 steps pass in about 83s, at 00:21.
- **How the examples went:** not one at a time this time. The first new
  example, "The doer and the validator", needed machines speaking ACP, the
  record, `watch` and the daemon all at once, and the rest of the
  homework's examples are those pieces seen from different sides. Once
  they were written, all 86 passed on the first full run. To check the new
  steps can fail, I broke the factory on purpose, one way at a time, and ran
  the three new feature files (32 examples). Each break made examples fail:
  - a machine's lines not passed on as they arrive (only its last, when
    its turn ends): 4 ("The doer is working", "Well into the run", ...);
  - no usage recorded: 4 (all of `monitoring.feature`, and "A run that has
    finished");
  - stopping a run doing nothing: 4;
  - a second run allowed: 1, "A second run while one is running";
  - a stopped run's work committed: 3 ("The doer is mid-attempt", "The
    reviewers are mid-review", "Starting the tetris run again");
  - no `running` lines: 1, "During fan out";
  - the run's folder not committed at the end: 3;
  - starting a run waiting until it ends: "Starting a run";
  - every usage recorded against one provider: "Three reviewers on three
    providers";
  - a stop shown as "ended": "The run is stopped while I watch";
  - the daemon exiting without stopping its run or answering `stop`: all 32,
    at the teardown that stops the factory.
- **Unit tests:** `internal/machine/harness_test.go`:
  `TestUnsignedHarnessFailsFast` (Review Focus 2, written before the ACP
  client: a fake `pi-rpc-acp` that exits 1 with "not authenticated", a
  machine with an empty folder; it failed first because the error didn't
  carry the harness's reason, and now fails in 2ms with "the doer
  (.../pi-rpc-acp) stopped before it answered (exit status 1): not
  authenticated"), `TestAHarnessThatSaysNothingFailsFast`,
  `TestAMachineSaysWhatItGeneratesLineByLine`, and homework 6's process
  tests, moved here and rewritten with harnesses that speak ACP
  (`internal/acptest`). `internal/watch/watch_test.go` checks the plain
  rendering. `TestRelativePathsResolveFromCallerCwd` and
  `TestAHangupStopsARunUnlessHangupsAreIgnored` now start a daemon, watch,
  and stop it.
- **Concurrency checks:** `go test -race` of `cmd/factory` and `internal/...`
  passes. The four feature files with fan out, the daemon and watching
  (`monitoring`, `observability`, `orchestration`, `fan-out`) also pass with
  the factory and doubles race-built (44 of 44, 425s, no data race;
  `logs/007-race-new-features.log`).
- **No process outlives its example:** after every default and race-built
  run, `ps` showed no daemon, `pi-rpc-acp`, `pi` or double left, and the
  daemons' folders were gone.
- **Time:** about 30 minutes from the fetch to a green suite (23:52 to
  00:21), then 30 more for the mutation checks, the race-built run and the
  README commands.
- **README commands**, through `bin/factory` from the repository root of a
  scratch repository holding copies of `tetris-007`'s `careful` and `quick`
  lines, with the agent double first on the `PATH` as `pi-rpc-acp`, and the
  doer held: the start printed `started run tetris` and returned; `status`
  showed the run; `watch` caught up (planner, then the doer's first line)
  and followed; the `snake` run on `snake/snake1` was refused with
  `factory: refused: run tetris is running`; `stop tetris` stopped it,
  leaving the doer's `alpha.txt` uncommitted, the plan as it was, and the
  run's folder committed; started again by name and target, the run
  finished (alpha and beta committed); `watch` of the finished run, with no
  daemon running, showed both invocations, the stop included; `stop`
  stopped the factory. The record was committed in the run's folder, and no
  login was committed.
- **Pi signed out, for real** (Review Focus 2): with the real `pi-rpc-acp`
  and pi, a machine with no login is stopped before any machine runs ("the
  planner cannot sign in to openai-codex"). One whose `auth.json` has an
  `openai-codex` entry with no tokens gets past that check; its run failed
  1.6s after the start with "the planner (pi-rpc-acp) ended its turn having
  generated nothing". Before the factory treated an empty turn as a
  failure, it said only "could not read the planner's result".

### Real-agent examples

`FACTORY_TAGS=@real-agent go test ./steps/ -run TestFeatures -count=1 -timeout 75m -v`,
with the pi wrapper first on the `PATH` (for the judge) and the starter's
`tools/pi-rpc-acp` on it, as a Codespace has it: 00:46 to 01:16 (**29m 51s**),
**9 of 9 pass**. Every machine ran as pi through the real `pi-rpc-acp`, and
every run was started by the daemon and watched to its end.

- "Reviewers that look at testability" 3m 26s; "Instructions above the
  target are not the machine's" 4m 06s; "A Tetris with different details"
  3m 02s; "The assembly line is given a seed and nothing else" 3m 28s; "The
  plan comes from the seed" 3m 40s; "A validator that looks at testability"
  3m 15s; "...at something else" 2m 26s; "The first task's work is
  untestable" 3m 07s; "A finding on the first task" 3m 18s.
- **Tokens**, from the runs' records, as `watch` totalled them: 41.4k to
  97.8k a run, 633.6k for the nine (inputTokens and outputTokens as the
  bridge reports them, which leave out cache reads). The judge's tokens
  aren't counted. Log: `logs/007-real-agent.log`.
- No daemon, `pi-rpc-acp`, `pi` or double was left running after the suite.

### Problems found

One in the starter, none in the tutorial:

- **`pi-rpc-acp` drops pi's errors.** When pi can't take a turn, its RPC
  mode ends the turn with `stopReason: "error"` and an `errorMessage` (for
  a login with no tokens, "No API key for provider: openai-codex"). The
  bridge passes over that and answers `session/prompt` with `end_turn` and
  no text, so the factory can't say why the machine said nothing. (With no
  login at all, pi refuses the prompt outright, and the bridge does pass
  that on.) The fix is seven lines in the bridge: remember the error at
  `turn_end`, and fail the prompt with it at `agent_end`. Checked with a
  scratch ACP client: the empty login now gives an error response with
  pi's message in 0.6s, no login gives the error as before, and a good
  login answers as before. It is committed on a local branch,
  `pi-rpc-acp-reports-pi-errors`, in a fresh clone of the starter
  (`/home/ensembleworks-agent/work/starter-fixes`, commit `bb5a9c7`). **The
  PR was not raised**: pushing to the public starter and opening the PR was
  refused by this session's permissions, so it waits for David's say-so.
  Until then the factory names the machine and harness of an empty turn
  itself (above), and this repository's copy of the bridge is left as the
  starter has it.

Things to note:

- pi-rpc-acp always runs pi with `--no-session`, so since this homework
  pi's sessions are no longer kept in the machine's folder (homework 3 left
  `--no-session` out to keep them). The run's record now keeps what each
  machine was given and generated.
- `bin/factory status` is a command the README doesn't list. The steps need
  a way to ask whether the factory and a run are running, and a daemon
  with no way to ask seemed worse than one more command.
- `watch` of a run that is running follows it until it ends; the steps for
  "I watch the run" read what it has shown so far.

### Fix round after review

The `homework-007` tag predates this fix. The review found a gap in
Review Focus 2: **a missing `pi` hung the run forever.** With `pi` off the
`PATH`, `pi-rpc-acp` answers `initialize` and `session/new`, takes the
prompt, and never answers it: its spawn error fires before a turn exists,
and is dropped. Neither the ACP client nor the start checked for it, so
the run said "started run tetris" and the machine waited until it was
stopped. A missing `pi-rpc-acp` was likewise only reported after the start.

Now, before any machine is called, the start checks each machine's
harness the way it checks its login. The harness must be on the caller's
`PATH`, or at the path its `machine.json` names. For `pi-rpc-acp`, `pi`
must be on the `PATH` too. Otherwise the start is refused, naming the
machine and what is missing: "could not run the planner (pi-rpc-acp): it
runs pi, and pi is not on the PATH". Through `bin/factory`, with the real
bridge and no `pi`, that took 0.6s.
`TestAMissingHarnessIsRefusedAtTheStart` starts a whole run with `pi`
missing (and a `pi-rpc-acp` that, like the real one, never answers), then
with `pi-rpc-acp` missing. Both are refused in under a second. With the
check taken out, both started, and the test failed. The suite's fake
`pi-rpc-acp` now has a fake `pi` beside it, which is never run.

The bridge itself should refuse the prompt with its spawn error too. That
is a second local commit on the starter branch above
(`pi-rpc-acp-reports-pi-errors`, `5530ca6`): with no `pi` on the `PATH`,
the prompt is refused with "could not run pi: spawn pi ENOENT" in 0.1s.
Like the first, it waits for David's go-ahead to be pushed.

Default suite 86/86, and `go test -race ./cmd/factory ./internal/...`
passes. The run path for a harness that is there is unchanged, so the
real-agent suite was not re-run.

## Homework 008: Steering

- **Adopted:** iteration `008-steering` from the tutorial's `main`, at 01:29.
- **Red:** with the adopted spec and homework 7's factory, 93 examples ran
  (the 9 `@real-agent` ones left out): 86 passed and 7 were undefined (1129
  steps: 1098 passed, 21 undefined, 10 skipped). The 7 are the examples of
  the new `steering.feature`. The only other changes are `FACTORY.md`'s
  paragraph on steering and the README. The failures were all in what the
  README calls new. No step-drift problem in the tutorial and no
  regression.
- **The target:** the README's commands name `tetris/tetris-008`, but
  steering changes nothing in a line, so there is no new copy of the lines
  in this repository. The suite still copies its lines from `tetris-007`.
  The README's commands were run in a scratch repository whose
  `tetris/tetris-008` held copies of `tetris-007`'s lines (below).
- **What changed in the factory:**
  - **`bin/factory say <run> --target <folder> <machine> <message>`**
    (`cmd/factory/main.go`) asks the daemon to pass the message to the
    machine, and returns once the machine's agent has taken it:
    `delivered to doer`. The flags can come anywhere among the arguments,
    and after `--` none is a flag, so a message can begin with `-`
    (`runArgs`, which `watch` and `stop` now use too).
  - **A message is for a machine that is working now** (`cmd/factory/run.go`,
    `daemon.go`). The run keeps which machines are taking a turn, in the
    order they began. A message for one that isn't, because it has finished
    or hasn't started, is refused, and nothing is kept for later:
    `refused: doer is what is running`, `refused: reviewer_1, reviewer_2 and
    reviewer_3 are what is running`, or `refused: nothing is running`
    (between machines, after the run, or with no factory running: then
    `refused: nothing is running: the factory is not running`). A machine
    the line doesn't have, or a run that isn't the one running, is refused
    saying so. Refusals exit 1 and, like homework 7's, are printed as
    `factory: refused: ...`.
  - **Steering** (`internal/machine/steering.go`). Each turn has a
    `machine.Steering`, given to `Machine.Run`. Once the prompt is sent,
    `Tell` sends `_session/steering` in the turn's session, with its
    `sessionId` and the message as a `prompt` of one text block, and waits
    (up to 10s) for the agent's answer. `{"outcome": "injected"}` is
    delivered. Anything else is refused, saying why: `promptRequired`
    (`noRunningTurn`, as the bridge answers once a turn is over), an error
    (an agent without the extension answers "Method not found"), or the
    turn ending before the agent answered. The ACP client now sends under a
    lock, and the turn, which reads all the agent's messages, passes a
    steering request's response to the `Tell` waiting for it.
  - **A message lasts as long as the turn that heard it.** It goes to that
    turn's session only. The next attempt and every other machine are new
    sessions, and the factory gives them nothing of it.
  - **The record** gets a `told` event (the machine and the message) when
    the agent has taken the message. `watch` shows it as
    `told       doer: use curses, not print`.
- **Doubles:** the agent double now goes on answering while it plays a
  turn. A `_session/steering` request during a turn is heard there and
  then: it says `heard: <message>`, which goes in `said/<n>.txt` like
  anything else it says, and answers `injected`. Outside a turn it answers
  `promptRequired`, as the bridge does. Something heard before the call has
  its number is kept once it has one (see the mutation checks).
- **Steps** (`steps/steering_steps_test.go`): "I say ... to the doer" runs
  `bin/factory say` and keeps what it said; "the doer finishes its
  attempt" and "the reviewers finish" let the held machines go on. A
  machine "heard" a message if a call to it said `heard: <message>`, or was
  given the message in its prompt, so a factory that passed messages on in
  later prompts would fail too. "Before it said" compares the order of the
  lines one call said. "The factory has run the run" runs it to the end, as
  "the factory runs the run" does. "It says that the doer is what is
  running" and "that nothing is running" read what `say` printed. "It
  shows that the doer was told" reads the record's `told` event.
- **Green:** 93 of 93 examples and 1129 steps pass in about 88s, at 01:46.
- **How the examples went:** one at a time. The first, "The doer is
  mid-attempt", failed once its steps were defined because the doer heard
  nothing; it needed the whole of steering (the command, the daemon's side,
  the ACP request and the double hearing). The other six passed as soon as
  their steps were defined: they are the same thing seen from other sides.
  To check those steps can fail, I broke the factory one way at a time and
  ran `steering.feature` (7 examples):
  - the message told to every machine running: 1 fails, "Three reviewers at
    once";
  - a message to a machine that isn't working delivered anyway (and
    dropped): 1, "The machine named has not started";
  - no `told` in the record: 1, "Reading back";
  - the message kept, and told to every later turn too: 2, "The next
    attempt has not heard it" and "The validator has not heard it either".
    This one at first broke nothing: a double told something before its
    call had its number didn't keep it. The double now keeps it;
  - the message kept, and given in every later prompt: the same 2;
  - `delivered` said without waiting for the agent's answer: none, as the
    double always takes a message. The unit tests below cover it.
- **Unit tests:** `internal/machine/steering_test.go`: a machine told
  something while it works is sent `_session/steering` with its session and
  the words, and `Tell` returns once the agent says `injected`;
  `promptRequired`, an error, or no answer before the turn ends are each
  refused, saying why (with `Tell` made to return at once, without the
  answer, these fail); a machine whose turn is over is told nothing.
  `internal/watch` checks the `told` line. `cmd/factory/say_test.go` checks
  the arguments, `--` included (it failed with `--` mishandled).
- **Concurrency checks:** `go test -race ./cmd/... ./internal/...` passes.
  `steering`, `monitoring` and `observability` pass with the factory and
  doubles race-built: 16 of 16, 128s, no data race
  (`logs/008-race-steering.log`).
- **No process outlives its example:** after every default and race-built
  run, `pgrep` found no daemon, `pi-rpc-acp`, `pi` or double left, and no
  daemon's folder was left in the temporary folder. (That folder is
  `$TMPDIR`, here `/var/tmp/bb-machines/...`, not `/tmp`. It holds 33
  daemons' folders from 00:36 to 00:37, before this homework began, with
  no daemon running for any of them: homework 7's deferred finding that a
  daemon killed with `kill -9` leaves its folder.)
- **README commands**, through `bin/factory` from the root of a scratch
  repository whose `tetris/tetris-008` held copies of `tetris-007`'s
  `careful` and `quick` lines (with stand-in logins), with the agent double
  first on the `PATH` as `pi-rpc-acp` and the doer held after saying
  `started`: the start printed `started run tetris`; `say tetris --target
  tetris/tetris-008 doer "use curses, not print"` printed `delivered to
  doer`; `say ... validator "be strict about error handling"` printed
  `factory: refused: doer is what is running` and exited 1; a machine not
  on the line, and a `say` with no message, were refused saying so;
  `watch` showed `doer | heard: use curses, not print` and the `told` line;
  let go, the run finished, and only the doer's first attempt (alpha) had
  heard it, not the second (beta) nor the validator; `say` then printed
  `factory: refused: nothing is running`, and after `stop`, `refused:
  nothing is running: the factory is not running`. No login was committed.
- **Steering pi, for real**, through the starter's `pi-rpc-acp` as it is on
  `main`: in a scratch repository, the `quick` line on a one-task seed that
  has the doer run `sleep 40` first. 8s into the doer's turn, `say greet
  --target tetris/tetris-008 doer "Also make a file kiwi.txt in the target
  that says kiwi, as part of this task."` printed `delivered to doer`, and
  `say ... planner hello` was refused (`doer is what is running`). The doer
  answered "Created `greeting.txt` and `kiwi.txt` after running `sleep
  40`", and its work commit held both files. The run took 72s. The same
  with the fixed bridge (below) added `mango.txt` the same way.
- **Time:** about 17 minutes from the fetch to a green suite (01:29 to
  01:46), then about 25 more for the mutation checks, the race-built run,
  the starter's bridge, the README commands and steering pi for real, and
  37 minutes of real-agent suite.

### Real-agent examples

`FACTORY_TAGS=@real-agent go test ./steps/ -run TestFeatures -count=1 -timeout 75m -v`,
with the pi wrapper first on the `PATH` (for the judge) and the starter's
`tools/pi-rpc-acp` as it is on `main`: 01:56 to 02:33 (**37m 06s**),
**9 of 9 pass**. `steering.feature` has no `@real-agent` example; these are
the earlier homeworks' nine, re-run because the ACP client changed.

- "Reviewers that look at testability" 4m 05s; "Instructions above the
  target are not the machine's" 4m 38s; "A Tetris with different details"
  3m 45s; "The assembly line is given a seed and nothing else" 5m 00s; "The
  plan comes from the seed" 4m 25s; "A validator that looks at
  testability" 4m 45s; "...at something else" 3m 18s; "The first task's
  work is untestable" 3m 29s; "A finding on the first task" 3m 40s.
- **Tokens**, the last total `watch` showed in each example: 53.5k to
  126.8k, about 750k for the nine (the judge's tokens aren't counted). Log:
  `logs/008-real-agent.log`.
- No daemon, `pi-rpc-acp`, `pi` or double was left running after the
  suite, and no daemon's folder was left.

### Problems found

Two in the starter's `pi-rpc-acp`, none in the tutorial. The bridge does
support `_session/steering`: given a message while pi works, it passes it
to pi's `steer` and answers `{"outcome": "injected"}`, and pi takes it in
before its next model call, whether it is running a tool or writing its
answer (checked with a scratch ACP client and the real pi: told a codeword
during a `sleep 8`, pi answered with it; told while writing, it added what
it was told). Once the turn is over it answers `promptRequired`
(`noRunningTurn`). But:

- **A message pi refuses is answered "injected", and kills the turn.** The
  bridge answers before pi has said whether it took the message. When pi
  refuses it (a steer that is one of pi's extension commands, say
  `/hello`), pi's refusal, a failed `response`, is taken as the prompt's
  failure: the client gets `injected` for the message and an error for the
  prompt, and pi works on unseen. Reproduced with an extension that
  registers `/hello`. The fix answers the steering request when pi answers
  its `steer`: `injected` if it took it, an error with pi's reason if not,
  and only a refused prompt fails the turn. With it, the refused `/hello`
  gets an error and the turn finishes; a codeword is still heard; a
  signed-out pi's prompt is still refused with its message.
- **The answer pi writes after a steer runs on from the one before.** pi
  writes a new assistant message after taking a steering message, and the
  bridge passed its text on with nothing between: "...a little
  longer.PINEAPPLE". The factory reads a machine's result as its last line
  of JSON, so a result line followed by a steered answer would be a line
  that isn't JSON. The fix starts a message on a new line when the last
  one's text didn't end one. Checked with a fake pi writing two messages
  ("first answer" then "second answer": one line through `main`'s bridge,
  two through the fixed one) and with the real pi.

Both are committed on a local branch, `pi-rpc-acp-steering-fixes`, from the
starter's `main` in `/home/ensembleworks-agent/work/starter-fixes`
(`b827b4d`, `8cfe8f6`). **Not pushed and no PR**: that waits for David's
go-ahead, like homework 7's `pi-rpc-acp-reports-pi-errors`. The two
branches change neighbouring lines of `onPiEvent`, so whichever merges
second needs a small merge. This repository's copy of the bridge is left
as the starter's `main` has it.

**The factory works with the bridge as it is on `main`**: the default
suite's doubles answer as the fixed bridge does, and the real steering
run above used `main`'s bridge. What `main`'s bridge gets wrong shows only
when pi refuses a message (then the factory says `delivered`, and the
machine's turn fails with pi's reason) or when a steered answer follows
a result on the same line (then, unless pi gives its result again, the
factory can't read it and says so).

Things to note:

- In what `watch` shows, a double's `heard:` line comes before the `told`
  line, because the factory records `told` once the agent has answered,
  and the double says what it heard before answering. A real pi shows
  nothing of hearing; it acts on it.
- The starter's bridge says in its opening comment that "homework 8's
  `run_command` tool" doesn't reach pi; its README says homework 9. Left
  alone.

## Beyond the homeworks: watch and --version

Not a homework: no iteration was fetched, and `factory/ITERATION` and
`factory/spec/` are as homework 8 left them. This makes `watch` look good
for a student's first ride on the finished factory, and gives the release
binary a version.

- **`watch` on a terminal** (`internal/watch`: `style.go`, `progress.go`,
  and `watch.go`). A `Watcher` styles its output only when it writes to a
  terminal (`golang.org/x/term`'s `IsTerminal`) and `NO_COLOR` is unset (and
  `TERM` isn't `dumb`). Otherwise the output is exactly homework 7's plain
  lines, with no escape and no `\r`, which is what the feature suite sees.
  - Each machine has a colour from a palette of six, in order of first
    appearance, kept for the run; names are padded to the longest machine
    name (the line's machine folders tell `watch` the names up front, so
    the column doesn't move mid-run; plain output widens the same way for
    a name over 10 characters, such as `synthesiser`).
  - A pinned bottom line, `▶ <line>  task <done>/<total>  <bar>  <elapsed>`,
    is wiped before each line and drawn again after it. The bar is 16 cells
    of `█` and `░`, `done * 16 / total` filled (5/12 is 6 cells). Without a
    valid `progress` it reads `▶ <line>  <elapsed>`. The clock ticks while
    nothing happens (`(*Watcher).Tick`, called by `follow` while it waits
    for the record to grow), and the line is cut to the terminal's width so
    it can't wrap. When the run ends the pinned line is replaced by
    `run tetris finished (6m 40s)`.
  - Token lines are dimmed, `running` lines are dim with the names in their
    colours, and a `told` line has a bold `told` and the machine in its
    colour.
  - `progress` in a result is `{"done": <int>, "total": <int>}`. A missing
    one, a non-object, a `total` of 0 or less, or values that aren't
    numbers are dropped (the last good progress stays); a `done` past
    `total`, or below 0, is held to 0 to `total`. In plain mode a change of
    progress prints as a line of its own: `progress   task 5/12  ██████░░░░░░░░░░`.
- **`factory --version`** prints `factory <version>` and exits 0. The
  version is `main.version`, set by `-ldflags "-X main.version=2026.11.03"`
  and `dev` otherwise. `-version` works too.
- **Fixed, from homework 7's deferred findings:** `Count` showed 999,950 as
  `1000.0k`. It now moves up a unit before it would round to a thousand
  (`1.0M`, and `G` after that). Its other outputs are unchanged.
- **Tests:** `internal/watch/render_test.go`: `TestWatchPlainWhenNotATerminal`,
  `TestNoColorMakesATerminalPlain` (a real pty), `TestProgressBarToleratesBadValues`,
  `TestProgressIsClamped`, `TestProgressBarCounts`, `TestPinnedLineStaysAtTheBottom`,
  `TestPinnedLineIsCutToTheTerminal`, `TestTickRedrawsTheClock`,
  `TestMachinesHaveStableColours`, `TestLongMachineNamesWidenTheColumn`,
  `TestNamedMachinesFixTheColumnFromTheStart`,
  `TestTerminalLooksRightForTokensToldAndTheEnd`,
  `TestElapsedIsShownInMinutesAndSeconds` and
  `TestCountChangesUnitBeforeItReachesAThousand`;
  `cmd/factory/version_test.go`: `TestVersionFlag`,
  `TestVersionOfADevelopmentBuild`.
- **Green:** the default suite passes whole: 93 of 93 examples, with every
  `watch` step unchanged (`logs/009-default.log`). No daemon, `pi-rpc-acp`
  or double was left running.
- **By hand, in a pty** (`script -qc ... /dev/null`), with the agent double
  behind a small proxy that makes the planner report `progress`: the
  `careful` line with three tasks, and `three-big-brains`. The pinned line
  advanced `task 0/3` to `task 3/3` with its clock, the three reviewers got
  three colours, and piped to a file the same record printed plain lines.
  The proxy is a scratch tool and isn't in the repository.
- **Fix after review: the terminal is restored when `watch` ends early.**
  The review found `watch` could end with the pinned line still on the
  screen: the "its record does not say how it ended" path, any error from
  `follow`, and Ctrl-C (so the shell's prompt landed on the pinned line).
  `(*Watcher).Close` now wipes the pinned line and draws it no more (once;
  nothing is written when plain or when nothing is pinned). `watch` closes
  the watcher before it says anything more, and stops on SIGINT, SIGTERM or
  SIGHUP (unless the signal was ignored at start, as with the rest of the
  factory), exiting 130. Two vacuous tests were rewritten too.
  `TestWatchEndedEarlyLeavesNoPinnedLine` runs `watch` in a pty and ends it
  with ^C, and by killing the daemon; with `Close` doing nothing, both fail.

## Beyond the homeworks: the ralph-loop line and the ride

Not a homework: no iteration was fetched, and `factory/ITERATION` and
`factory/spec/` are as homework 8 left them. This writes the line a student
rides before building anything: a planner and a doer, built for speed, and
proves it on Tetris with the real `pi`.

- **The line** (`lines/ralph-loop/`): `start -> planner`,
  `planner -> doer [label="not complete"]`, `planner -> finish
  [label="complete"]`, `doer -> planner`. No validator, no fan out. Each
  machine has `AGENTS.md`, `machine.json` (`pi-rpc-acp`, `openai-codex/gpt-6-sol`)
  and the `.gitignore` that keeps everything else out of Git. The planner's
  instructions are the quick line's (keep an existing plan's tasks; leave a
  task open while its subtasks are) with a plan written as a checklist of
  eight to twelve small tasks, and the result
  `{"complete": <bool>, "progress": {"done": <n>, "total": <n>}}`. The
  doer's add "work fast" and "leave the product working, run its tests".
  `TestTheRalphLoopLineIsAPlannerAndADoer` and
  `TestTheRalphLoopPlannerKeepsThePlanAndReportsProgress` check the graph,
  the model and the rules.
- **`bin/ride <target> [--run name] [--seed file]`** copies the line into
  `<target>/.assembly-lines/ralph-loop` (leaving the target's runs alone),
  copies `~/.pi/agent/auth.json` into each machine, puts `tools/pi-rpc-acp`
  on the `PATH` and starts the run with `${FACTORY_BIN:-bin/factory}`. The
  seed is `tetris/spec.md` unless given. `RIDE_MODEL` replaces the model
  both machines name (used for the comparison below). No login is
  committed: `git ls-files | grep -i auth` finds nothing.
- **Parsed first**, with a fake `pi-rpc-acp` first on the `PATH`:
  `bin/ride /tmp/probe --run probe` started, `watch` showed the planner's
  `progress 1/1` and `run probe finished`, then `factory stop`.

### Choosing the model

Each ride is a fresh target under `tetris/`, run for real with `pi` on the
build box's `openai-codex` login, `watch`ed from a log under `logs/`. Wall
time is from the start of the run to its last commit; tokens are the last
total `watch` showed (in + out, the cache not counted). "Playable" is
`npm --prefix <t> start` in a pty, driven by a script that presses the
arrow keys, space and `q` (so a game that dies on its first key is not
playable), plus the game's own tests.

| Target | Model | Planner's text | Wall time | Tokens | Tasks | Own tests | Playable |
|---|---|---|---|---|---|---|---|
| `ride-sol` | gpt-6-sol | first | 7m 11s | 126.1k | 10 | 5 pass | yes (24 rows) |
| `ride-luna` | gpt-6-luna | first | 5m 20s | 186.3k | 11 | 5 pass | yes (24 rows) |
| `ride-kill` | gpt-6-luna | first | 44s | 23.1k | 10 | none | no: the doer wrote a 3-line `index.js`, then the planner marked all 10 tasks done and `complete` |
| `ride-sol-2` | gpt-6-sol | + check before marking | 7m 23s | 151.3k | 10 | 4 pass | yes (22 rows, alternate screen, cursor restored) |
| `ride-luna-2` | gpt-6-luna | + check before marking | 5m 21s | 189.7k | 11 | 4 pass | **no**: dies on the first key (`key.toLowerCase is not a function`: stdin was never given an encoding) |
| `ride-kill-2` | gpt-6-sol | + check before marking | 7m 49s (killed once) | 166.2k | 10 | none | yes |

(`ride-kill` was meant for step 4; its run was over before there was
anything to kill, so `ride-kill-2` was used.)

- **gpt-6-luna is about two minutes faster and uses 25% to 50% more tokens,
  and two of its three rides were not a working game.** `ride-kill` showed
  the planner marking tasks done that the doer hadn't done; the planner
  now says "don't take anyone's word that a task is done, the doer's
  included... A task the doer hasn't done stays open, however much of the
  product already exists" (a general rule, on the line's planner only).
  With it, luna's ride finished, but its game crashed on the first key
  pressed, and its own four tests passed because none of them presses a
  key. Both of gpt-6-sol's rides, and the killed one, were playable. Sol
  moved the bar one task at a time; luna's moved two at a time at times.
- **Picked: `gpt-6-sol`**, the fastest model that was playable every time.
  One ride a model is a small sample (the first Luna ride was fine); the
  numbers above are what was seen, not a ranking.
- **All the ride targets are scratch** (`tetris/ride-sol`, `ride-sol-2`,
  `ride-luna`, `ride-luna-2`, `ride-kill`, `ride-kill-2`): they show what
  the line makes and nothing depends on them. They sit inside this
  repository, so the factory committed their work and their run folders
  here, as it does for any target; their copies of the line are committed
  too, as each ran it (`ride-sol` and `ride-luna` ran the planner text
  before the check was added). Nothing else reads them.

### The bar

On a pty (`script -qc "stty cols 100 rows 30; ... watch ride --target
tetris/ride-sol"`) the pinned line went `task 0/10 ░░░░...`, `task 1/10
█░░░...`, `task 2/10 ███░░░...` and on to `task 10/10`, with the clock
ticking between. The planner reports `progress` on every pass; once, with
`ride-kill`'s luna planner, its first result left it out and the pinned line
read `▶ ralph-loop  <elapsed>` until the next one, as designed.

### An interrupted ride (Review Focus 5)

`bin/ride tetris/ride-kill-2 --run ride`, then `kill -9` of the daemon in the
doer's third turn (`task 2/10` on the bar).

- **The daemon alone, killed:** no `.git/index.lock` was left (its `git`
  children finish without it), and no `pi-rpc-acp` or `pi` outlived it (their
  stdin closed). `watch` said `run ride is not running, and its record does
  not say how it ended`. The doer's half-done work stayed uncommitted in the
  target; the plan was already committed.
- **Started again, the same command:** the run resumed. The planner was
  given the plan that exists, kept its 10 tasks and reported `2/10`; the
  doer did task 3 on top of the half-done work, and the run went on to
  `10/10` and `run ride finished`. The plan's tasks before and after were
  the same ten (diffed with the ticks removed). `factory stop` left no
  daemon behind.
- **Where it can go wrong: Git itself killed.** A `pre-commit` hook that
  sleeps, with `git commit` killed while it held the lock (the whole process
  group, as a Codespace going to sleep would do), left `.git/index.lock`.
  Every `git add` and `git commit` after it failed, so a restarted run died
  at its first commit (the planner's "Update the plan"). **Fixed**: a commit
  that finds the lock in its way waits for it to go, or removes it when no
  `git` process is running to hold it, and tries again; if one is still
  running after 10s it fails saying so and leaves the lock. With a lock
  planted before the restart, the real resume above cleared it and went on.
  Tests (`git_test.go`): `TestAStaleIndexLockDoesNotStopACommit` (failed
  before the fix, with the lock's "File exists" error) and
  `TestALiveGitsIndexLockIsLeftAlone`.
- **The planner's commit had no test.** `TestARunInterruptedAfterThePlannersCallHasThePlanCommitted`
  (`resume_test.go`) kills the daemon with `kill -9` while the doer works,
  checks `Write the plan` is in Git with the planner's plan, starts the run
  again and checks the planner is given that plan (not told there is none).
  With the commit in `run.go` removed it fails, on both checks.
  Both fixes are in their own commit before the line's.
- **Green:** the default suite passes whole (`logs/010-default-final.log`).

### Problems found

- `TestWatchEndedEarlyLeavesNoPinnedLine/the_factory_killed` (homework 9's
  polish) leaves its `sh` harness running after the suite: three were found
  on the box, one from each default-suite run, the oldest from before this
  task. They hold no files open that matter, but a suite that must leave
  nothing running doesn't. Not fixed here; for the final review.
- The play script and the pty watch are scratch tools
  (`/home/ensembleworks-agent/work/ride-check/play.py`); they aren't in the
  repository.

## Beyond the homeworks: the release build

Not a homework: no iteration was fetched, and `factory/ITERATION` and
`factory/spec/` are as homework 8 left them. This builds the binaries a
first-time student's Codespace downloads for its ride.

- **`bin/release <version>`** takes a date version, `YYYY.MM.DD` (`.1`, `.2`
  for a second or third release the same day), and refuses unless the tree
  is clean, the version has that shape *and* names a real date (month 1 to
  12; day within the month, February 29 only in a leap year), and the tag
  and `dist/<version>/` don't exist yet. It then builds `linux-amd64`,
  `linux-arm64` and `darwin-arm64` with `CGO_ENABLED=0 go build -trimpath
  -ldflags "-s -w -X main.version=<version>"` into `dist/<version>/` as
  `factory-<version>-<os>-<arch>`, writes `SHA256SUMS` (bare file names, so
  `sha256sum -c SHA256SUMS` works wherever they land), tags this repository
  `<version>` (lightweight, like `homework-00N`), and prints the `gh release
  create` command without running it. Nothing is uploaded. `dist/` is in
  `.gitignore`; no binary is committed. `bin/factory` is not touched.
- **One flag beyond the brief: `-buildvcs=false`.** Go otherwise stamps the
  commit into the binary, so the same source builds different bytes once
  the commit changes (here, after amending the log). Without the stamp the
  same source and flags always give the same `SHA256SUMS`, which was
  checked: the build was redone after the commit was amended and the three
  sums were identical.
- **Static, on purpose.** `net` is the only package in the factory's
  dependencies with cgo files, and it uses it only for DNS and user
  lookups; the factory dials a unix socket and nothing else, and imports
  neither `os/user` nor `net/http`. With `CGO_ENABLED=0` `file` says
  `statically linked` for both Linux binaries and `ldd` says `not a dynamic
  executable`. (Built without `CGO_ENABLED=0`, the same code is dynamically
  linked against `ld-linux`, which is why the flag matters.)
- **Step 1, refusals.** Dirty tree: refused. `2026.13.01`: no month 13.
  Also refused, each with a message naming what is wrong: `2026.00.10`,
  `2026.02.29` (not a leap year), `2100.02.29` (a century that isn't), `2026.04.31`,
  `2026.10.00`, `2026.10.32`, and the wrong shapes `2026.1.1`, `v2026.10.26`,
  `2026.10.26.x`, `2026.10.26.`.
- **Step 2, the build of `2026.10.26`:** three binaries (about 4.2 MB each)
  and `SHA256SUMS` in `dist/2026.10.26/`. `file`: `ELF 64-bit LSB
  executable, x86-64 ... statically linked, stripped` and `ELF 64-bit LSB
  executable, ARM aarch64 ... statically linked, stripped`, and `Mach-O
  64-bit arm64 executable`. `factory-2026.10.26-linux-amd64 --version`
  prints `factory 2026.10.26`. It takes about 30 seconds.
- **Not run on this box:** the `linux-arm64` and `darwin-arm64` binaries.
  The release notes say so and ask a Mac user to try `--version` and a ride.
- **Not published.** The `gh release create` command is printed by
  `bin/release` and left for David.

### Step 3: the release binary rides Tetris

`FACTORY_BIN=dist/2026.10.26/factory-2026.10.26-linux-amd64 bin/ride
tetris/release-ride --run ride`, from the repository root, with the real
`pi` on the box's `openai-codex` login and `watch ride` (the release binary
too) logging to `logs/011-ride-watch*.log`. Another scratch target, like the
other ride targets; the factory committed its work and its run folder here.

- **The first attempt failed at task 4**, 2m 40s in and 49.2k tokens spent:
  `run ride failed: could not read the doer's result: no line of its answer
  is a JSON object`. The doer's last answer put its result on the end of its
  prose line (`...gravity behavior. {"task":"..."}`) where its three earlier
  answers had it on a line of its own after a blank one. It is not the
  release build's doing: the dev build reads results the same way. It is a
  model slip the factory is strict about, and a student can meet it.
  (Stated plainly in the report, not fixed here.)
- **Started again with the same command, the run resumed**: the planner
  kept its 10 tasks, said 3 of 10 were done (the doer's fourth task was not
  yet accepted), and the run went on to `task 10/10` and `run ride
  finished`, 5m 54s later. About 8m 30s of machines' time in all; the run
  total `watch` showed at the end was 170.7k tokens (in + out, cache not
  counted). The factory's own commits run from `Start the ride run` to
  `Keep what the ride run made`: one for each task, with an `Update the
  plan` after each.
- **What it made**: 189 lines of JS in `src/` (`game.js`, `index.js`,
  `pieces.js`, `render.js`), a README, and 6 tests under `node --test`, all
  passing. Its `npm start`, driven in a pty by the same script as the
  earlier rides (left, left, up, space, right, down, then `q`): 16 frames,
  at most 23 rows, alternate screen and hidden cursor on, restored on exit,
  `q` exits with status 0. Playable by that test; nobody has played it by
  hand yet. It is in `tetris/release-ride` (`npm --prefix
  tetris/release-ride start`, controls: arrows move, up rotates, down soft
  drops, space hard drops, `r` restarts, `q` quits).
- **Cleanup**: `factory stop` stopped the daemon (the daemon stays up after
  a run by design, and the run said "not running"); `pgrep` found no daemon,
  `pi-rpc-acp` or `pi` after.

## Final review

The whole-branch review's findings, fixed after the `2026.10.26` build.
No `homework-*` tag and not the `2026.10.26` tag were moved, and
`factory/spec/` is as homework 8 left it.

- **Results after prose, on the last line, are read.** The release ride's
  first attempt failed when the doer ended its answer with `...behavior.
  {"task":"gravity"}`. Two fixes. The ralph-loop machines' `AGENTS.md` now
  say: "End with a blank line, then the result alone on the last line:
  nothing before it on that line and nothing after it." And the factory is
  more lenient than the homework text, which asks for a line of JSON on
  its own: on the last non-blank line only, if the line isn't itself a JSON
  object, it takes the line from the first `{` that starts one through to
  the end of the line (`see {x} then {"complete":true}` gives
  `{"complete":true}`). Why: a model that puts its result after a sentence
  has answered, and failing a student's first ride on that slip costs more
  than reading it. Everything else is as before: an answer in prose with no
  JSON object still fails with the same message (`orchestration.feature:202`
  and `fan-out.feature:105` pass), a JSON line on its own is taken as it
  was, and a JSON line above a prose last line is still taken.

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

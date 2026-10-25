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

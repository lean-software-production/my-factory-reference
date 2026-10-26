# A student's first use of the reference factory: design

Status: draft for David's review, 2026-10-26. It replaces `2026-10-25-first-ride-design.md`.

## Why

A self-paced student's first hands-on time with the course should show them the factory they're about to build, working. Before homework 1, they use the finished factory from `my-factory-reference` to build a Tetris, watch it work, and play the result. The course videos explain what's behind it, so this step doesn't stop to explain.

The student types the factory's commands themselves. The status bar needs a real terminal, and the commands are the ones their own factory will answer by homework 7. Their coding agent gets things ready and tells them what to type.

The word "ride" belongs to the course's metaphor (building the bike and riding it), not to files, folders or commands (David, 2026-10-26).

## What the student does

1. **Setup**, as the starter's README has it now: fork, Codespace, sign in to Codex and Pi, then `bin/doctor --agent codex`.
2. **They tell their coding agent:** "use the reference factory to build tetris-000" (David's wording).
3. **The agent gets ready:**
   - It runs `bin/get-reference-factory`, which downloads the pinned release to `bin/reference-factory` and checks it.
   - It makes sure the two machines, `planner` and `ralph` in `tetris/tetris-000/.assembly-lines/implement-with-ralph-loop/`, can use the provider the student's Pi is signed in to. Pi needs to be authenticated with something, but it needn't be `openai-codex`: an API key such as `OPENROUTER_API_KEY` will do. The course recommends `openai-codex` because its limits are permissive for this kind of use (David, 2026-10-26).
     - If Pi is signed in with a login, the skill copies it (`~/.pi/agent/auth.json`) into both machine folders.
     - If the student uses an API key in the environment, nothing is copied. The factory already accepts a provider's key in the environment for the providers in its table, `OPENROUTER_API_KEY` among them.
     - Both machines' `machine.json` name `openai-codex/gpt-6.1-sol` (David, 2026-10-26). If that isn't the student's provider, the skill sets both to a model of the student's provider. It offers one from `pi --list-models <provider>` and lets the student choose, and says the timing in the README was measured on the default model.
   - It shows the student two commands to type in a terminal:
     ```
     bin/reference-factory --run tetris --target tetris/tetris-000 --line implement-with-ralph-loop --seed tetris/spec.md
     bin/reference-factory watch tetris --target tetris/tetris-000
     ```
4. **The student types them.** `watch` shows the log above a three-row status bar: the line, `start ▸ planner ▸ ralph ↺ planner` with the round, then time and tokens. A run took about 7 minutes on `gpt-6-sol`; both machines now default to `openai-codex/gpt-6.1-sol` (David, 2026-10-26), and the next release's ride measures it.
5. **They play:** `npm --prefix tetris/tetris-000 start`.

That's the end of this step. The agent doesn't follow up with "this is what you'll build" or offer "fetch iteration" (David, 2026-10-26: step 6 dropped). The README carries on to homework 1 as it does now.

## What changes

### In `my-factory-reference` (first)

1. Done on 2026-10-26: `watch-status-bar` (the status bar, the line renamed `implement-with-ralph-loop` with its doer `ralph`, and the `gpt-6.1-sol` default) is merged into `main` and pushed (`e4c336e`).
2. Rename `bin/ride` to `bin/use-reference-factory`, with the same steps. Its `RIDE_MODEL` override becomes `MODEL`, and its default target becomes `tetris/tetris-000`, matching the starter. Add a note to the build log.
3. Release a new version with `bin/release`, ride it once on a fresh Tetris target and check it's playable. David plays it and publishes it on `my-factory-starter` with the command `bin/release` prints. The starter can't use the new flow until that release exists.

### In `my-factory-starter` (one PR, kept a draft until the release is published)

It's tested first against the locally built release, through the URL override, and then against the published one.

- **`tetris/spec.md`**: the Tetris seed, a byte-for-byte copy of the tutorial's `docs/iterations/001-basic-unvalidated-loop/spec.md`, which is the only homework that ships a seed. The starter's `tetris/` holds only `.gitkeep` today, and the seed must exist before homework 1 is fetched. `fetch.sh` copies the seed only if `tetris/spec.md` doesn't exist yet, so shipping the same text changes nothing for homework 1. If the tutorial ever changes 001's seed, the starter's copy has to change with it.
- **`tetris/tetris-000/.assembly-lines/implement-with-ralph-loop/`**: a copy of the reference repo's `lines/implement-with-ralph-loop/`, with its `assembly-line.dot` and `machines/{planner,ralph}/` holding `AGENTS.md`, `machine.json` and the `.gitignore` that keeps logins out. Nothing else is in `tetris-000` until it's built.
- **`bin/get-reference-factory`**:
  - It holds the pinned version in one line near the top, so updating the course to a new release is a one-line change.
  - It picks the asset for the machine it runs on (`linux-amd64`, `linux-arm64` or `darwin-arm64`) and downloads it, along with `SHA256SUMS`, from that release on `lean-software-production/my-factory-starter`.
  - It checks the binary against `SHA256SUMS`, installs it as `bin/reference-factory` (executable), and prints `factory --version`.
  - Run again, it does nothing if the right version is already there.
  - It needs no login, because the release is public.
  - For testing before a release is published, the release's base URL can be overridden with an environment variable, so the script can be pointed at a locally served `dist/<version>/`.
  - On `linux-arm64` and `darwin-arm64` it says those builds haven't been tried yet, and carries on.
- **`.gitignore`**: `bin/reference-factory`.
- **A skill, `use-reference-factory`**, in `.agents/skills/` (and so `.claude/skills/`). It triggers on "use the reference factory to build tetris-000" and similar requests, and does step 3 above. Like the other skills, it's written for the agent. It checks before it shows the commands:
  - `bin/reference-factory --version` runs;
  - the provider both machines name can be reached: `pi auth check --provider <provider> --no-refresh` says ready, or that provider's API key is in the environment;
  - with a login, both machines now hold an `auth.json`.

  If any check fails, it stops and says what to do. On a repeat request it looks at the run first. If `tetris-000`'s run has finished, it says so and gives the play command. If the run was interrupted, the same two commands resume it. It doesn't start a second build.
- **`AGENTS.md`**: the new skill in its list.
- **`README.md`**: a new step between "Start your coding agent" and "Fetch Homework 1", with the student's phrase, what they'll see (with "You should see" lines in the README's style), and the play command. The "You're set up when" checklist gains a line for having built and played `tetris-000`.

### `tetris-000` stays in Git (David, 2026-10-26)

The factory commits a target's work and its run folder when the target is inside a repository. So the Tetris that `tetris-000` builds, and its run record, become commits in the student's fork before homework 1. That is intended.

### In the self-paced drafts (parent thread)

- "Meet your AI tutor" ends with this step, or links to it.
- "Run a Ralph Loop (or two)" uses `tetris-000` instead of the separate ralph-loop-demo Codespace.

## Non-goals

- No steering, and no looking behind the scenes in this step (David, 2026-10-25).
- The line has no validator and no fan out. It's built for speed (David, 2026-10-25).
- The student's own factory isn't touched: `bin/factory` and `factory/` are as they were.

## Left to the builder

- The skill's exact wording, within the steps above.
- `bin/get-reference-factory`'s implementation: `curl` or `gh`, and `sha256sum` or `shasum`.
- The README step's wording, in the README's existing style.

## Calls made in this design (not yet confirmed by David)

- The phrase triggers a skill of its own. "coach me" doesn't start this step: the README tells the student to say the phrase before "fetch iteration".
- `bin/get-reference-factory` is a script, not part of the skill, so a student or instructor can run it by hand.
- The new name for `bin/ride` in the reference repo (`bin/use-reference-factory`) and its `MODEL` override.
- The starter ships `tetris/spec.md`, a copy of homework 1's seed.
- The arm64 builds ship untried, and say so when they're used. Codespaces are `linux-amd64`.
- The skill checks the binary, the provider's sign-in and the copied logins before it shows the commands, and handles a repeat request without starting a second build.
- The starter PR stays a draft until the release is published.
- For a provider other than `openai-codex`, the skill changes both machines' model itself, with the student choosing from that provider's models, rather than telling the student to edit `machine.json`.

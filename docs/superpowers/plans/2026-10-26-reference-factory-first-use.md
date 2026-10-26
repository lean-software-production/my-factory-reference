# A student's first use of the reference factory: implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A student who forks `my-factory-starter` can say "use the reference factory to build tetris-000". Their coding agent gets the published reference factory and readies the line, and the student types two commands to build and watch a Tetris.

**Architecture:**
- **The reference repo.** It renames `bin/ride` and cuts a release that has the status bar, the line `implement-with-ralph-loop` and the model `gpt-6.1-sol`.
- **The starter.** It ships the seed and the line in `tetris/tetris-000/`, a download script `bin/get-reference-factory` that installs `bin/reference-factory`, and a skill that does the agent's part.

**Tech Stack:** bash, `curl`, `sha256sum` or `shasum`, Go (reference repo only), and Agent Skills `SKILL.md` files.

**Spec:** `docs/superpowers/specs/2026-10-26-reference-factory-first-use-design.md` (copy it in from `/home/ensembleworks-agent/.bb/thread-storage/thr_ahpnztqd9i/specs/` in Task 1).

## Global Constraints

- **No "ride" in files, folders or commands** in either repo. The word belongs to the course's metaphor only.
- **The student's phrase** is "use the reference factory to build tetris-000".
- **The student's two commands**, verbatim:
  - `bin/reference-factory --run tetris --target tetris/tetris-000 --line implement-with-ralph-loop --seed tetris/spec.md`
  - `bin/reference-factory watch tetris --target tetris/tetris-000`
- **The play command** is `npm --prefix tetris/tetris-000 start`.
- **The default model** for both machines is `openai-codex/gpt-6.1-sol`.
- **Pi needs any sign-in:** a login in `~/.pi/agent/auth.json`, or a provider's API key in the environment (for example `OPENROUTER_API_KEY`). `openai-codex` is the recommendation, not a requirement.
- **Releases** are published from `lean-software-production/my-factory-starter`. Assets are `factory-<v>-linux-amd64`, `factory-<v>-linux-arm64`, `factory-<v>-darwin-arm64` and `SHA256SUMS`.
- **Never publish a release.** Only print the command. David publishes.
- **The reference repo.**
  - Work on branch `use-reference-factory`, then merge it into `main` and push. David approved the design, which covers that.
  - Don't move any existing tag.
- **The starter.** Work on branch `use-reference-factory`. Open a **draft** PR, and never push `main`.
- **Commits** end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. PR bodies end with `🤖 Generated with [Claude Code](https://claude.com/claude-code)`.
- **Write plainly**, without slogans. Follow each repo's `AGENTS.md`.

## Review Focus

1. **A student with an API key and no Pi login.** Nothing breaks. No `auth.json` is copied, and the run uses the key from the environment. Tests: Task 1, `bin/use-reference-factory_test.sh` "no login, key in env"; Task 3, the skill's text, checked in review.
2. **A course update to a new version.** `bin/get-reference-factory` replaces an older `bin/reference-factory`. Test: Task 3, "an older binary is replaced".
3. **A corrupt or partial download.** It's refused, and nothing is installed or left half-written. Test: Task 3, "a checksum mismatch installs nothing".
4. **An unsupported machine** (an Intel Mac, `darwin-amd64`). A clear message naming the supported builds, with exit 1. Test: Task 3, "an unsupported platform is refused".
5. **A repeat request after `tetris-000` was built.** The skill gives the play command and doesn't start a second build. Test: Task 3's end-to-end check, step 8.

---

### Task 1: Rename `bin/ride` in the reference repo

**Files** (repo `/home/ensembleworks-agent/my-factory-reference`, branch `use-reference-factory` from `main`):
- Rename: `bin/ride` to `bin/use-reference-factory`.
- Create: `bin/use-reference-factory_test.sh`.
- Create: `docs/superpowers/specs/2026-10-26-reference-factory-first-use-design.md` (a copy), and add this plan.
- Modify: `docs/build-log.md`, by appending a section.

**Interfaces:**
- Produces `bin/use-reference-factory [<target>] [--run <name>] [--seed <file>]`. The defaults are target `tetris/tetris-000`, run `tetris` and seed `tetris/spec.md`. It reads `FACTORY_BIN` (the default is `bin/factory`) and `MODEL`, which replaces the model both machines name; `RIDE_MODEL` is gone. If `~/.pi/agent/auth.json` exists it copies it into each machine with mode 600; if not, it carries on and copies nothing. Its help text and echo contain no "ride".

- [ ] **Step 1:** Write `bin/use-reference-factory_test.sh` in the style of the starter's `bin/doctor_test.sh`, with a scratch `HOME` and a fake `FACTORY_BIN` that writes its arguments to a file. It covers four cases:
  - **Defaults:** the arguments are exactly `--run tetris --target tetris/tetris-000 --line implement-with-ralph-loop --seed <repo>/tetris/spec.md`, and `tetris/tetris-000/.assembly-lines/implement-with-ralph-loop/machines/{planner,ralph}/machine.json` exist in a scratch copy.
  - **With a login:** `auth.json` is in both machines with mode `600`.
  - **No login, key in env:** no `auth.json` is in either machine, and the fake factory was still called.
  - **`MODEL=openrouter/some-model`:** both `machine.json` name it.

  Point the script at a scratch copy of the repo's `lines/` and `bin/` so the real `tetris/` is untouched.
- [ ] **Step 2:** Run `bin/use-reference-factory_test.sh`. Expected: it fails, because there's no `bin/use-reference-factory`.
- [ ] **Step 3:** `git mv bin/ride bin/use-reference-factory`, and make the changes in Interfaces.
- [ ] **Step 4:** Run the test again. Expected: all pass. Run `cd factory && go test ./... -count=1`. Expected: all green. `grep -rn -i ride bin/` should find nothing but the build log's history.
- [ ] **Step 5:** Append "Renamed: bin/use-reference-factory" to the build log. Commit as `Rename bin/ride to bin/use-reference-factory, with tetris-000 as its default`, merge into `main`, run the suite, and push `main`.

### Task 2: Release the reference factory and use it once

**Files:** `docs/build-log.md`, in the reference repo, on `main`.

**Interfaces:**
- Consumes `bin/release` and `bin/use-reference-factory` (Task 1).
- Produces a tag `<V>`, where `<V>` is today's CalVer. `2026.10.26` and `2026.10.26.1` already exist, so on 2026-10-26 it's `2026.10.26.2`. It also produces `dist/<V>/`, and the printed `gh release create` command, which Task 3 and David use.

- [ ] **Step 1:** With a clean tree on `main`, run `bin/release <V>`. Expected: three binaries plus `SHA256SUMS`, and `sha256sum -c` is OK. `dist/<V>/factory-<V>-linux-amd64 --version` prints `factory <V>`.
- [ ] **Step 2:** Build Tetris with the release binary, outside the repo so `main` stays clean: `FACTORY_BIN=$PWD/dist/<V>/factory-<V>-linux-amd64 bin/use-reference-factory /home/ensembleworks-agent/builds/<V> --seed tetris/spec.md`.
  - Run it in the background, with a log under `/home/ensembleworks-agent/work/reference-first-use/`.
  - Check `watch` in a pty shows the status bar, `start ▸ planner ▸ ralph ↺ planner` with a round.
  - The run should finish complete. Check the game is playable with `python3 -I /home/ensembleworks-agent/work/ride-check/play.py <target>`.
  - Record the wall time and tokens on `gpt-6.1-sol`.
  - Stop the factory afterwards.
- [ ] **Step 3:** Append a "Release <V>" section to the build log: the time, the tokens, playable or not, and the printed `gh release create` command. Commit, then push `main` and the tag `<V>`. **Don't run `gh release create`.**

### Task 3: The starter: seed, `tetris-000`, `bin/get-reference-factory`, the skill and the README

**Files** (repo `/home/ensembleworks-agent/capstone-project-starter`, branch `use-reference-factory` from `main`):
- Create:
  - `tetris/spec.md`, byte-identical to the tutorial's `docs/iterations/001-basic-unvalidated-loop/spec.md`;
  - `tetris/tetris-000/.assembly-lines/implement-with-ralph-loop/`, holding the reference repo's tracked `lines/implement-with-ralph-loop/` files only: `assembly-line.dot` and `machines/{planner,ralph}/{AGENTS.md,machine.json,.gitignore}`;
  - `bin/get-reference-factory`;
  - `bin/get-reference-factory_test.sh`;
  - `.agents/skills/use-reference-factory/SKILL.md`. `.claude/skills` is a symlink to it already.
- Delete: `tetris/.gitkeep`.
- Modify:
  - `.gitignore`: add `bin/reference-factory`.
  - `AGENTS.md`: the skill in its list.
  - `README.md`: see step 6.

**Interfaces:**
- **Consumes:** release `<V>` and `dist/<V>/` from Task 2.
- **Produces `bin/get-reference-factory`:**
  - `version=<V>` is on one line near the top, and is the only place the version appears.
  - The release base URL defaults to `https://github.com/lean-software-production/my-factory-starter/releases/download/$version`. `REFERENCE_FACTORY_URL` overrides it, and `file://` works for tests.
  - It maps `uname -s`/`uname -m` to `linux-amd64` (Linux x86_64), `linux-arm64` (Linux aarch64 or arm64) or `darwin-arm64` (Darwin arm64). Anything else exits 1, naming the three supported builds.
  - It downloads the asset and `SHA256SUMS` to a temp dir and checks the sum with `sha256sum` or `shasum -a 256`. Only on a match does it move the binary into `bin/reference-factory` (chmod +x). On a mismatch it exits 1 and installs nothing.
  - If `bin/reference-factory --version` already prints `factory <V>`, it says so and exits 0 without downloading.
  - On arm64 it prints that this build hasn't been tried yet.
  - It ends by printing `bin/reference-factory --version`.
- **Produces the skill `use-reference-factory`:** its frontmatter `description` says to use it when the student says "use the reference factory to build tetris-000", or asks to use or try the reference factory.

- [ ] **Step 1:** Write `bin/get-reference-factory_test.sh` in `bin/doctor_test.sh`'s style. It serves a fake release from a scratch folder through `REFERENCE_FACTORY_URL=file://…`: a fake `factory-<V>-linux-amd64` script that prints `factory <V>`, plus a matching `SHA256SUMS`. A fake `uname` comes first on `PATH`. The cases:
  - **"installs and checks":** `bin/reference-factory --version` prints `factory <V>`, and the file is executable.
  - **"a checksum mismatch installs nothing":** with a wrong sum, it exits 1 and there's no `bin/reference-factory`.
  - **"already there, no download":** after an install, delete the served binary and run again. It exits 0 with an "already" message.
  - **"an older binary is replaced":** install a fake that prints `factory 2000.01.01`, run the script, and get `factory <V>`.
  - **"an unsupported platform is refused":** with `uname` faked to `Darwin x86_64`, it exits 1, the output names `linux-amd64, linux-arm64, darwin-arm64`, and nothing is installed.

  Run it against a scratch copy of `bin/`, so the real `bin/reference-factory` is never touched.
- [ ] **Step 2:** Run the test. Expected: it fails, because there's no script.
- [ ] **Step 3:** Write `bin/get-reference-factory` as in Interfaces. Run the test. Expected: all pass. `bin/doctor_test.sh` and `python3 .agents/skills/fetch-iteration/fetch_test.py` still pass.
- [ ] **Step 4:** Add `tetris/spec.md` and the line. Run `cmp` against the tutorial's 001 `spec.md`, and `diff -r` against the reference repo's `lines/implement-with-ralph-loop` on tracked files. Remove `tetris/.gitkeep`, and add the `.gitignore` line.
- [ ] **Step 5:** Write the skill. Its steps, for the agent:
  1. If `bin/doctor` isn't ready, stop and say to fix setup first.
  2. Run `bin/get-reference-factory`.
  3. Find the student's sign-in and get the machines ready:
     - With a login, `~/.pi/agent/auth.json`: copy it into both machines with mode 600.
     - With only an API key in the environment, copy nothing.
     - If the provider of `openai-codex/gpt-6.1-sol` isn't reachable (`pi auth check --provider openai-codex --no-refresh`, no API key for it), find the provider they are signed in to. Offer models from `pi --list-models <provider>`, let them choose, and set both `machine.json` to it. Say the README's timing was measured on the default model.
  4. Check: `bin/reference-factory --version` runs, the machines' provider is reachable, and with a login both `auth.json` exist. If a check fails, stop and say what to do.
  5. If `tetris/tetris-000/.assembly-lines/implement-with-ralph-loop/runs/tetris/` exists:
     - **finished:** the record ends with the run finished. Say it's built, and give the play command;
     - **otherwise:** say the same two commands resume it.
  6. Show the two commands, verbatim, for the student to type in a terminal at the repository root, and say they can watch for about seven minutes. When the run is complete, give the play command.

  Rules: never type the commands for the student, don't edit `factory/`, and don't fetch an iteration.
- [ ] **Step 6:** The README.
  - Add a new `### 4. Build tetris-000 with the reference factory`, and renumber "Fetch Homework 1" to 5, fixing every link and reference to step numbers. The new step covers: the phrase, as a chat message; what the agent does; the two commands; a "You should see" with the status bar's route line; and the play command.
  - Add a checklist line: "You've built `tetris/tetris-000` with the reference factory and played it."
  - In "Where things live", add `bin/get-reference-factory`, `bin/reference-factory` (git-ignored) and `tetris/tetris-000/`, and say `tetris/spec.md` is now in the starter.
  - In "Testing the starter", add the new test.
  - In `AGENTS.md`, add the skill line in the existing list's style.
- [ ] **Step 7:** End to end, against the local release. In a temp clone of this branch, follow the skill by hand:
  - `REFERENCE_FACTORY_URL=file:///home/ensembleworks-agent/my-factory-reference/dist/<V> bin/get-reference-factory`;
  - copy the login;
  - type the two commands (the run in the background), with `tools/pi-rpc-acp` on `PATH` as in a Codespace.

  Expected: the status bar shows `start ▸ planner ▸ ralph ↺ planner`, the run completes, `play.py` passes, and `git log` in the clone shows the factory's commits for `tetris/tetris-000`. Record the time.
- [ ] **Step 8:** In the same clone, run the skill's step 5 check by hand. It finds the finished run and gives the play command.
- [ ] **Step 9:** Commit in logical pieces:
  1. the seed and the line;
  2. the script and its test;
  3. the skill, `AGENTS.md` and the README.

  Push the branch, and open a **draft** PR against `main`. Its body says it waits for release `<V>` to be published, how it was tested (the local release, the end-to-end run's time), and that the arm64 builds are untried.

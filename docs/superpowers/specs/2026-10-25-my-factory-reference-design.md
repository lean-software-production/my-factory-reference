# my-factory-reference: design

Status: draft for David's review, 2026-10-25. Real runs use the build box's `openai-codex` Pi login (David, 2026-10-25).

## Why

The self-paced course wants a student's first hands-on experience to be riding the factory they are going to build: start a run, watch it work, play the Tetris it made. The course overview video and the first unit are about showing students where the course goes before they write any code. That needs a working factory that does everything the tutorial's eight homeworks describe, shipped as one binary a Codespace can download and run.

This repo builds that factory. It is a student's fork of `my-factory-starter`, worked through homework 001 to 008 the way a student would, in Go. Its source is meant to be private, because it is a finished answer to every homework; only the binary is meant to be public. For now the repo stays public.

Building it is also the first full run of the renumbered tutorial (tutorial#9, starter#13) from 001 to 008. Anything broken in a homework, a feature file or a starter skill should show up here before a student meets it.

## What gets built

- **`lean-software-production/my-factory-reference`**, meant to be private (public for now; see Constraints). Its history starts from `my-factory-starter`'s `main`, so the starter's skills, `bin/doctor` and `tools/pi-rpc-acp/` are there as a student would have them.
- **A Go factory in `factory/`**, built one homework at a time:
  1. Run `fetch-iteration`.
  2. Make the homework's suite pass, leaving out the `@real-agent` examples, using the starter's `implement-fast` skill.
  3. Commit with the skill's message (`Implement homework 00N`).
  4. Tag the commit `homework-00N`.

  At 008 the factory has everything in `FACTORY.md`: configured machines, assembly lines in the target, named runs, fan out, a daemon you can `watch`, and steering with `say`.
- **The step definitions and test doubles**, written in Go as the homeworks ask. Students write their own doubles (tutorial ADR 0002), and so does this repo.
- **A polished `watch`.** The homework 7 behaviour, presented as option A from the 2026-10-25 mockups:
  - lines coloured and labelled by machine;
  - a bottom line showing the line's name, the progress so far as a bar, and the elapsed time;
  - a token line by machine and for the run.

  The features still decide what `watch` must do. This is presentation on top.

  The factory never reads the plan (`planning.feature`), so `watch` can't count tasks itself. Instead, the progress comes from a machine's result. If a result carries a `progress` field, say `{"done": 4, "total": 12}`, `watch` shows it; if not, the bar is left out. The `ralph-loop` line's planner is told in its `AGENTS.md` to report it.
- **The `ralph-loop` line for the ride**: a planner and a doer, with no validator and no fan out, for `pi`. It's written and tested here against the real binary. The ride project then copies it into the starter, which ships it. Its graph and machines' `AGENTS.md` are course material, but students don't build them in any homework.
- **Release binaries**: `linux-amd64`, `linux-arm64` and `darwin-arm64`, each a single static binary with no runtime to install.

## How it's released

- **Manually, to a public release on `lean-software-production/my-factory-starter`.** Forks don't copy releases, so the ride downloads from the upstream starter.
- **CalVer tags**, `YYYY.MM.DD`, with `.1`, `.2` added for a second or third release on one day. The homework commits stay as a student would make them. The `watch` polish, `--version` and the `ralph-loop` line come in commits after `homework-008`. Each release is built from a commit tagged with its version in `my-factory-reference`, and its notes name that commit.
- **Asset names include the version and platform**, for example `factory-2026.11.03-linux-amd64`, so the ride can pin one exactly.
- **`factory --version`** prints the CalVer version, so a student or the coach can see which one they have.

## Constraints

- **Built as a student would build it.** No change to `factory/spec/`. It is fetched, not edited, and a homework counts as done when its suite passes. If a homework can't be done as written, that's a tutorial bug: fix it in the tutorial, not here.
- **Never at `bin/factory`.** The student's own `set-up-factory` creates that symlink. The ride installs the reference binary somewhere else; the ride's design says where.
- **The source is meant to be private, but stays public for now.** The repo is a public fork of the starter, and the GitHub App can push to it. David chose on 2026-10-25 to leave it public for now. A fork's visibility can't be changed, so making it private later means recreating it as a non-fork repo and pushing its history in.
- **Machines run `pi` by default**, through `pi-rpc-acp` from homework 7. A student's Codespace has Pi signed in to their ChatGPT plan, and the ride uses that.
- **A real run before every release.** The suites run with doubles and leave out `@real-agent`, so before each release the `ralph-loop` line runs for real on Tetris, through `pi` with the `openai-codex` login on the build box, and the result must be playable. (Decided by David, 2026-10-25.) The `@real-agent` examples also run once per homework, on the same login.
- **Go is already installed** on the build box (`go1.27.3`).

## Non-goals

- **Not a model answer to publish.** Its code is for us, and maybe for instructors comparing a student's factory. Students never see it.
- **Not tuned beyond the homeworks**, apart from the `watch` presentation and `--version`. The ride should show students the factory they will build, not a better one.
- **No automated release pipeline yet.** Releases are uploaded by hand. CI can come later if releases get frequent.

## Left to the builder

- **The Go code.** Its layout, libraries and Gherkin runner (godog is the obvious one), and how the daemon and `watch` talk to each other, within what the features allow.
- **Fixing tutorial problems.** How to fix any problems found along the way, raised as PRs against the tutorial or the starter. Until a fix merges, carry on by fetching from its branch with `COURSE_REF`. A fix to a homework already done applies from the next fetch; earlier tags aren't redone.

## Calls made in this design (not yet confirmed by David)

- Go rather than Rust. David said "GoLang or perhaps Rust"; Go builds faster, is easier for students to read when they compare, and cross-compiles from one machine.
- `implement-fast` drives the build, and each homework is tagged `homework-00N`.
- The CalVer format, `YYYY.MM.DD[.N]`, and the asset names.
- `factory --version`.
- Progress in `watch` comes from a `progress` field in a machine's result, so the factory still never reads the plan.
- The `ralph-loop` line is developed in this repo, and the starter ships a copy.
- The polish, `--version` and the line go after `homework-008`; releases are built from version tags.
- Tutorial fixes: carry on from the fix branch with `COURSE_REF`, and don't redo earlier homeworks.
- Running each homework's `@real-agent` examples once, on the build box's `openai-codex` login.

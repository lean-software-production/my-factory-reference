# Homework 5 — Runs and targets

Read `FACTORY.md`, then the feature files in `features/`. Together they are
the whole spec of the factory at this point, not just the new parts.

Your factory already builds into the target folder you choose. Until now
every invocation supplied a seed and target and used the same assembly line. Each
target kept one plan: an implicit run for that generation. This homework
introduces named runs that remember what they build and how, while the
factory source stays in `factory/`.

- **The assembly lines move into the target.** A target holds its
  product and, in `.assembly-lines/`, the lines that build it: a folder
  each, named for the line, holding its `assembly-line.dot` and its
  machines in `machines/`. Each line has its own machines, so two lines
  can each have a doer, configured differently. Move your line folder
  from homework 4 into `tetris/tetris-005/.assembly-lines/` (or whichever
  target you choose).
- **Each execution is a named run, kept on its line.** You name the run,
  and the first time you start it you give it one of the target's lines
  and a seed. The run remembers both. It lives in its line's folder, in
  `runs/<name>/`, so you always say which target it is in: its target and
  its name are enough to find it. Its plan is `plan.md` there. To resume
  your existing game, move its `.factory/plan.md` into the run's folder;
  a run now owns that plan. Or start with a fresh target.
- **The run's folder is committed.** The factory commits it in commits
  of its own, never with the work, so a run's plan and record stay in
  Git with what it built. A machine's login never is: when you move or
  copy a line, its machines' logins come with it, and Git still ignores
  them.

```text
tetris/tetris-005/
  .assembly-lines/
    careful/
      assembly-line.dot
      machines/
        planner/    AGENTS.md, its configuration and its login
        doer/
        validator/
      runs/
        tetris/
          plan.md
          ...       whatever the machines keep for the run
    quick/
      assembly-line.dot
      machines/
        planner/
        doer/
      runs/
  src/ ...          the product
```

The seed can be anywhere: it is the run's input, not part of it. The run's
folder is where the run's own things go. Tell your machines, in their
`AGENTS.md`, to keep anything they make for the run there: notes,
artifacts, logs. The factory tells each machine where the folder is.

Once the route is data, a target can hold more than one line. Write a second
one — the same line without the validator will do — and have each run say
which line it runs.

Starting or resuming a run is an **invocation** of that same run. Its name,
settings and plan persist across invocations.

## The commands you'll end up with

The exact shape is your choice. Run from the repository root, with
`bin/factory` linked to your entry point:

```sh
# a new run: its target, its line and its seed
$ bin/factory --run tetris --target tetris/tetris-005 --line careful --seed tetris/spec.md

# after that, its name and its target are enough
$ bin/factory --run tetris --target tetris/tetris-005

# a second target: write snake/spec.md describing Snake first
# copy a line into it, machines included, but no runs
$ mkdir -p snake/snake1/.assembly-lines
$ cp -r tetris/tetris-005/.assembly-lines/quick snake/snake1/.assembly-lines/
$ rm -rf snake/snake1/.assembly-lines/quick/runs
$ bin/factory --run snake --target snake/snake1 --line quick --seed snake/spec.md
```

The factory should build whatever the new seed describes, without disturbing
the first run.

The checks still run against a copy of your factory, which now keeps
nothing of its own about runs. The copy builds in separate targets, each
holding its own lines, their machines and their runs.

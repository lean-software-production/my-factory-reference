# The factory, as of this iteration

The factory builds software from a seed. A planner writes the plan, then
a doer makes it one task at a time and a validator checks each attempt.
Once a task's work is committed, the planner marks it done, and it says
when the plan is complete.

The factory writes none of its machines' work itself, and never reads
the plan. A machine has a name and a folder of its own on its line,
which is the configuration of the harness that runs it, `pi` by default
or another named there. The folder holds the machine's `AGENTS.md`, its
instructions, and the rest of its configuration, such as its model. It
also holds the machine's own copy of your login, which is kept
out of Git; with no login, a machine uses its provider's API key. The
harness runs on that folder's configuration and no other: not yours, and
none found in or above the target, whose files are the machine's work.
Each machine answers with a result: JSON describing the job it did.

The route through the machines is an **assembly line**: a graph the
factory reads before it does any work. An edge only routes: its label
names a field of the result of the machine it leaves. The planner
decides whether there is more to do; the line holds the loop back to it
and the retry after failed validation, and the factory limits how many
attempts a task gets. Lines live in the target, in `.assembly-lines/`,
a folder each, named for the line: its graph, `assembly-line.dot`, its
machines, each in a folder of its own in `machines/`, and its runs, in
`runs/`. A target can hold several lines, and each has its own machines.
A line never names a target.

A **run** is a named execution of one of the target's assembly lines on a
seed, saying what to build, against a target, the codebase to build it in. The
factory source is separate from the generated product. Targets
are plain folders: commits in their containing repository include only
the selected target's work and preserve unrelated staged changes. Git is
initialized only when no repository contains the target. The run lives in its line's folder, in `runs/<name>/`, with its plan and whatever
its machines keep for it, and the factory commits that folder in
commits of its own, never with the work. A run is found by its target
and its name, so every invocation names the target; nothing of a run is
kept in the factory.

New since iteration 4: runs, and targets that hold their own lines, each
with its machines and its runs. A run's plan moves from the target's
`.factory/` into its folder on its line, which is committed.

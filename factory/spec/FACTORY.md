# The factory, as of this iteration

The factory builds software from a seed. A planner writes the plan, then
a doer makes it one task at a time and a validator checks each attempt.
Once a task's work is committed, the planner marks it done, and it says
when the plan is complete.

The factory writes none of its machines' work itself, and never reads
the plan. A machine has a name and a folder of its own on the line,
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
attempts a task gets. The factory has one assembly line, a folder of
its own in `assembly-lines/`: its graph, `assembly-line.dot`, and the
machines on it, in `machines/`.

Each run requires both a seed file and a target folder. Relative paths
are resolved from the caller's working directory; absolute paths work too.
Targets are plain folders. The factory creates a missing folder, uses its containing Git repository and initializes
Git only if no repository contains it. It runs agents in the target and
commits only that target's work and plan, preserving unrelated staged
and unstaged edits. Each target keeps its own plan in `.factory/plan.md`. A fresh target starts fresh; rerunning a
target resumes its plan. An omitted or missing seed reports that there is
no seed, without calling an agent. Before stopping successfully, the
factory records the final plan, including updates after a work commit.
A completed run leaves no uncommitted changes in the target.

New since iteration 3: `assembly-line.feature`. The pass is gone — the
line says what runs next — and the machines move onto the line, each
in its own folder in the line's `machines/`.

# The factory, as of this iteration

The factory builds software from a seed, one task at a time. A planner
keeps the plan, a doer does each task and a validator checks it. The
validator only reports; the doer records each finding and tries again,
and the pass gives up after a set number of attempts. Once the work is
committed, the planner marks the task done, and it says when the plan is
complete.

The factory writes no project code, no plan and no verdict itself, and
never reads the plan. Its three machines do — the planner, the doer and
the validator. Each has a folder of its own in the factory's folder,
`machines/<name>/`, which is the configuration of the harness that runs
it, `pi` by default or another named there. The folder holds the
machine's `AGENTS.md`, its instructions, and the rest of its
configuration, such as its model. It also holds the machine's own copy
of your login, which is kept out of Git; with no login, a
machine uses its provider's API key. The harness runs on that folder's
configuration and no other: not yours, and none found in or above the
target, whose files are the machine's work. Each machine answers with a
result, a line of JSON describing the job it did, and the factory reads
the planner's and the validator's results to decide what happens next.

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

New since iteration 2: each machine has a folder of its own, its
harness's configuration, with an `AGENTS.md` and a login of its own. The factory holds no
instructions of its own for any machine, and each machine names its own
harness.

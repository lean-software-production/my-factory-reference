Feature: Planning

  How the planner makes the plan from the seed, and keeps it true.

  Background:
    Given a copy of the factory
    And a new target, with a seed describing a game of Tetris
    And the target has an assembly line "careful" on which the doer's work is validated
    And a run named "tetris", on the "careful" line, with that seed and target
    And the planner plans the tasks alpha and beta
    And the doer does the next task in the plan
    And the validator is always satisfied

  Rule: The seed is the assembly line's only input

    What to build comes from the seed alone. The assembly line is given
    the seed and nothing else.

    @real-agent
    Example: The assembly line is given a seed and nothing else
      Given every machine runs pi
      When the factory runs the "tetris" run
      Then Tetris has been built in the target

  Rule: A run's seed must exist

    Example: The seed has gone
      Given the seed has been deleted
      And no plan
      When the factory runs the "tetris" run
      Then it reports that there is no seed
      And no agent has been called
      And there is no plan

  Rule: The planner writes the plan before any work is done

    Example: A seed with no plan yet
      Given no plan
      When the factory runs the "tetris" run
      Then the planner was called before the doer
      And the plan shows every task as done

    Example: A plan already exists
      Given a plan with three tasks, none of them done
      When the factory runs the "tetris" run
      Then the plan still has those three tasks

    @real-agent
    Example: The plan comes from the seed
      Given every machine runs pi
      And no plan
      When the factory runs the "tetris" run
      Then every task in the plan comes from the seed

  Rule: A run keeps its plan in its line's runs folder, in the target

    Each run has a folder of its own, in the runs/ folder of the line it
    runs, and its plan is plan.md there. Nothing of a run is kept in the
    factory.

    Example: The first invocation of a run named "tetris"
      Given no plan
      When the factory runs the "tetris" run
      Then the plan is plan.md in the "careful" line's runs folder, under tetris
      And there is no plan in the factory's folder

  Rule: The planner keeps the plan, and the factory never reads it

    The planner writes the plan. Once a task's work is committed, the
    planner marks it done, and its result says whether any task is
    left. The doer works from the plan too. The factory only knows whether
    there is a plan, and the machines' results.

    Example: A run carries on from its last invocation
      Given a plan whose first task is done
      When the factory runs the "tetris" run
      Then the plan shows every task as done
      And there are two new work commits

    Example: Work that gave up is not recorded
      Given a plan with three tasks, none of them done
      And the factory allows at most three attempts at a task
      And the validator is never satisfied
      When the factory runs the "tetris" run
      Then the plan shows every task as not done

    Example: A plan no factory could parse
      Given the planner keeps its plan in prose
      And the doer keeps its plan in prose
      And no plan
      When the factory runs the "tetris" run
      Then the work for alpha and beta has been committed

  Rule: A run remembers its assembly line and its seed

    The line and the seed are given when a run starts. After that, its
    name and its target are enough. The run is kept in its target, so the
    target is named every time.

    Example: The "tetris" run, run again by name
      Given the "tetris" run has been started
      And a plan whose first task is done
      When the factory runs the "tetris" run, given only its name and target
      Then there are two new work commits
      And the validator has been called twice

  Rule: A run's settings cannot be changed once it has started

    Naming a run again with its original line and seed is fine, and so is
    naming it with its target alone. Naming it with a different line is
    refused: a run's name is unique in its target, whichever line it
    runs.

    Example: The "tetris" run is given a different line
      Given the "tetris" run has been started
      And the target has an assembly line "quick" on which the doer goes straight to the planner
      When the factory runs the "tetris" run on the "quick" line
      Then the factory refuses
      And it says the "tetris" run already has a line

    Example: The "tetris" run is given the same settings again
      Given the "tetris" run has been started
      And a plan with three tasks, none of them done
      When the factory runs the "tetris" run
      Then there are three new work commits

  Rule: Each run has its own plan and its own target

    Example: Two runs, one after the other
      Given a new target, with a seed describing a game of Snake
      And the "careful" line has been copied into the target
      And a run named "snake", on the "careful" line, with that seed and target
      When the factory runs the "tetris" run
      And the factory runs the "snake" run
      Then each run has its own plan
      And each target holds only its own run's work

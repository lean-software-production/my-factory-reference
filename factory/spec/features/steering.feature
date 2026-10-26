Feature: Steering

  Talking to a machine while it works. The factory passes the message to
  the running machine with ACP's _session/steering request, as the ACP
  adapters for Claude Code and Codex accept it, and the machine picks it
  up at its next step.

  A machine an example sees at work, part-way through or in a run just
  started, goes on working until the example says it finishes, or the run
  is stopped or run to the end.

  Background:
    Given a copy of the factory
    And a new target, with a seed describing a game of Tetris
    And the target has an assembly line "careful" on which the doer's work is validated
    And a run named "tetris", on the "careful" line, with that seed and target
    And the planner plans the tasks alpha and beta
    And the doer does the next task in the plan
    And the validator is always satisfied

  Rule: A message reaches the machine at its next step

    Example: The doer is mid-attempt
      Given a plan with one task, not done
      And the doer says "started", then "finished"
      And the "tetris" run is running, with the doer part-way through an attempt, having said "started"
      When I say "use curses, not print" to the doer
      And the doer finishes its attempt
      And the "tetris" run has run to the end
      Then the doer heard "use curses, not print" before it said "finished"
      And there is one new work commit

  Rule: A message names the machine it is for

    More than one machine can be working at once, so there is no such
    thing as "the machine that is running". A message names a machine
    that watching shows as running.

    Example: Three reviewers at once
      Given the three big brains has replaced the validator on the "careful" line
      And every reviewer is always satisfied
      And the synthesiser is always satisfied
      And a plan with one task, not done
      And the "tetris" run is running, with every reviewer part-way through
      When I say "be strict about error handling" to reviewer_2
      And the reviewers finish
      And the "tetris" run has run to the end
      Then reviewer_2 heard "be strict about error handling"
      And neither reviewer_1 nor reviewer_3 heard it

  Rule: The factory refuses a message to a machine that is not running

    Example: The machine named has not started
      Given a plan with one task, not done
      And the "tetris" run is running, with the doer part-way through an attempt
      When I say "be strict about error handling" to the validator
      Then the factory refuses
      And it says that the doer is what is running

    Example: Nothing is running
      Given a plan with one task, not done
      And the factory has run the "tetris" run
      When I say "use curses, not print" to the doer
      Then the factory refuses
      And it says that nothing is running

  Rule: A message lasts as long as the run that heard it

    Example: The next attempt has not heard it
      Given a plan with one task, not done
      And the validator is not satisfied the first time
      And the "tetris" run is running, with the doer part-way through an attempt
      When I say "use curses, not print" to the doer
      And the doer finishes its attempt
      And the "tetris" run has run to the end
      Then the doer has been called twice
      And the doer heard "use curses, not print" once

    Example: The validator has not heard it either
      Given a plan with one task, not done
      And the "tetris" run is running, with the doer part-way through an attempt
      When I say "use curses, not print" to the doer
      And the doer finishes its attempt
      And the "tetris" run has run to the end
      Then the validator did not hear "use curses, not print"

  Rule: A message is part of the record

    Example: Reading back
      Given a plan with one task, not done
      And the "tetris" run is running, with the doer part-way through an attempt
      When I say "use curses, not print" to the doer
      And the doer finishes its attempt
      And the "tetris" run has run to the end
      And I read the "tetris" run's record
      Then it shows that the doer was told "use curses, not print"

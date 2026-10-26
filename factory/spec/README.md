# Homework 8 — Steering

Read `FACTORY.md`, then the feature files in `features/`.

You can see what the machines are doing. Now you can answer them. While a
machine is working you can send it a message, and it picks the message up
at its next step — the way you interrupt an agent mid-task, not the way
you edit its prompt.

Two things make this harder than it sounds. More than one machine runs at
once during fan out, so a message names the machine it is for. And a
message is for a machine that is working now: if the machine you name has
finished, or has not started, the factory tells you so rather than holding
your message for whoever comes next.

A message lives in that machine's run and dies with it. If you want
something to stick, it belongs in the seed, the plan, or the machine's own
prompt. Noticing where the difference bites is most of the point of this
homework.

Stable ACP has no way to reach a running turn yet. The factory uses
`_session/steering`, an extension the Claude Code and Codex adapters
already accept, and the pi bridge too: a request with the `sessionId` and
a `prompt`, answered `{"outcome": "injected"}` when the message went in.

## The commands you'll end up with

On top of homework 7's, one more — again, the exact shape is yours:

```sh
$ bin/factory say tetris --target tetris/tetris-008 doer "use curses, not print"
delivered to doer

$ bin/factory say tetris --target tetris/tetris-008 validator "be strict about error handling"
refused: doer is what is running
```

## Doubles that hear

A double that is being steered shows what it heard in what it generates,
at the point it heard it, so a check can tell whether a message arrived
before or after something the machine said.

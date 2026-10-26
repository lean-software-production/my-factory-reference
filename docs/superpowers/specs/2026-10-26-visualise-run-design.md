# Replaying a run in the browser: design

Status: approved by David, 2026-10-26.

## Why

David demos the reference factory to students before they build their own. He wants them to see that the agents are **guided through a workflow rather than going wild**, and that the factory is a real thing, without being overwhelmed. That shows best on a busier line, with reviewers fanning out and work sent back (David, 2026-10-26).

`watch` is a terminal view of a live run. For a demo, David wants a browser page that **replays a finished run from its record**, so he can fast-forward and show the same run every time (David, 2026-10-26).

## What it is

A new command in the reference factory:

```
factory visualise-run <path>/record.jsonl
```

It reads the run's record and its line's graph, serves a page from the factory itself, and prints the page's address. The page replays the run.

- **Hidden** (David, 2026-10-26): it ships in the release students download, but it isn't in `factory --help`, in the usage text, or in the starter's README. It works for anyone who knows its name.
- `watch` doesn't change. There's no live mode: the page replays a record as it is when the command reads it.

## What the page shows

Two mockups show the approved look:
- [The demo line](2026-10-26-visualise-run-mockup-3-big-brains.html): David's `implement-with-3-big-brains` run at 9m 31s, with real log lines from its record. It shows the layout, the models, results as short lines, and the timeline's break.
- [`three-big-brains`](2026-10-26-visualise-run-mockup.html): the bubble placement and tails. Its working machines flash, which has since been dropped (see "The machines").

Both graphs are placed by hand.

### The drawing

- The run's assembly line drawn as its graph, left to right, from `start` to `finish`, in the brand's Sketchbook style: wobbly boxes and arrows, Patrick Hand lettering, paper background. It uses the colours and fonts of the [brand kit](https://github.com/lean-software-production/brand/tree/main/kit) and nothing else.
- Every edge is drawn, with its label (`not complete`, `complete`, `not satisfied`, `satisfied`).
- Edges that go back (to a machine earlier in the route) are blue. The others are slate.
- An edge the run hasn't taken yet, up to the moment shown, is dotted. Once taken, it's solid.
- A small round badge on each edge counts how many times the work has gone that way so far. The badge appears after the first time. On a back edge, it's blue.
- No connector moves or animates (David, 2026-10-26).
- `start` and `finish` are drawn as rounded pills with dashed outlines.
- Above the drawing: `<target> · <line> · run "<run>"` on the left, and `round <n>` on the right.

### The machines

- **Working:** a steady yellow fill (a mix of the kit's mustard and paper) with a darker mustard outline. No flashing (David, 2026-10-26).
- **Done in this round:** a pale green fill (forest mixed with paper) and a tick after the name.
- **Not reached in this round:** plain paper.
- Several machines can be working at once (the three reviewers).
- Under each machine's name, in smaller text, is the model it used, from its most recent `usage` event: the model without its provider, and the reasoning level after a dot when the model names one (`openai-codex/gpt-6.1-sol:high` shows as `gpt-6.1-sol · high`). Before a machine's first `usage` event, the line is blank (David, 2026-10-26).
- Machine names are shown with spaces for underscores (`plan reviewer opus`).
- A round starts each time the planner starts. The planner is the machine with an edge to `finish`, found from the graph, as `watch` finds it. Starting a new round clears the ticks.

### The logs

- Clicking a machine opens its log in a speech bubble (the kit's bubble) whose tail points at that machine (David, 2026-10-26).
  - It opens above the machine with the tail pointing down. If there isn't room above in the window, it opens below with the tail pointing up.
  - It's kept inside the window. If that pulls the tail off the machine, the tail is hidden.
  - It follows the machine when the window is resized or scrolled.
- The bubble's heading is the machine's name and `working` or `last time`.
- Each line is a piece of text the machine generated (the record's `text` events), stamped with the run's elapsed time (`6:31`). The bubble scrolls, and keeps to its newest lines.
- The machine's result (its `result` event) shows as one short line instead of the raw JSON: true and false fields as `✓` and `✗` (`satisfied ✓ · retry ✗ · replan ✗`), then a `task` or `summary` field if there is one, cut short, then, when the result has a non-empty `findings` list, how many there are and the first one (`1 finding: Unknown escape sequences can dispatch embedded bytes as game commands.`), so a send-back says why (David, 2026-10-26). Other fields are left out. The JSON text the result was read from isn't shown again. The record sometimes runs a sentence and the JSON together on one `text` line, and the sentence is still shown.
- Most of what a machine says arrives when it finishes, so while it works its bubble mostly shows what it said in earlier turns. Tool calls aren't recorded today, and showing them is a later piece of work (David, 2026-10-26).
- While the replay plays, an open bubble keeps filling, and its newest line is highlighted.
- A machine that isn't working shows what it generated the last time it worked.
- One bubble is open at a time. Its ✕ or a click on the same machine closes it. A click on another machine moves it there.
- Machine text is shown as text, never as HTML.

### The bar

At the foot of the page:

- Play and pause.
- Speeds: 1×, 10× and 60×. 10× is selected when the page opens.
- A timeline across the whole run, filled up to the moment shown, with a tick for each hand-off (each time a machine starts). A send-back (a machine starting along a back edge) has a blue tick. Clicking or dragging on the timeline jumps there.
- "Next hand-off", which jumps to just before the next machine starts.
- The elapsed time out of the total (`6m 40s of 15m`) and the run's tokens so far (`284k tokens`), formatted as `watch` formats them.

### Opening and ending

- The page opens **paused on the first frame**, so David can talk before he presses play.
- At the end, `finish` lights up (forest), every machine stops, and a line under the drawing says the run finished, with its time and tokens. If the record ends without the run finishing, the line says it stopped, and when.

## Which runs it must replay well

1. A `tetris-000` build on `implement-with-ralph-loop` (`planner` and `ralph`). The reference repo has three ralph-loop records under `tetris/release-ride*/`, on the line's old name and with its doer called `doer`.
2. **David's demo run** on a busier line he provides, with its record, both made by the reference factory (David, 2026-10-26). Its graph:
   - `start → plan_gate`, which routes `pending` to `doer`, `ready` to `planner` and `not ready` to `plan_author`;
   - `plan_author` fans out to three reviewers, `plan_reviewer_opus`, `plan_reviewer_gemini` and `plan_reviewer_astra`, which meet at `plan_synthesiser`, and that sends the plan back to `plan_author` (`not satisfied`) or on to `doer` (`satisfied`);
   - `doer → validator`, which sends work back to `doer` (`retry`) or to `plan_author` (`replan`), or on to `planner` (`satisfied`);
   - `planner` goes to `doer` (`not complete`) or `finish` (`complete`).

   It's the run the demo is built around, so it must read well. David has provided a finished run (copies in BB thread storage, `thr_ahpnztqd9i/demo-line/`: `assembly-line.dot` and `record.jsonl`). What's in it:
   - the line is called `implement-with-3-big-brains`, on target `tetris/tetris-007`, run `tetris-007`, made in a Codespace (paths start `/workspaces/`). The line was first called `efficient`, so the record's prompts name that folder;
   - one plan round: `plan_gate` (not ready), `plan_author`, the three reviewers at once, `plan_synthesiser` (satisfied), then 13 tasks, each `doer` then `validator` then `planner`, until the planner says `complete` and the run finishes;
   - a stop and a resume: the run was stopped during the planner at 17:25, and started again 19 minutes later, when `plan_gate` said `ready` and went straight to `planner`;
   - **two send-backs**: the validator sent tasks 11 and 12 back to the `doer` (`retry`), with findings such as "Unknown escape sequences can dispatch embedded bytes as game commands", and passed each on the next attempt. No `replan` or `not satisfied` happens;
   - 51 hand-offs, about 614k tokens, and 36m 41s of work once the 19-minute gap is cut out;
   - `plan_gate`'s `usage` events have a model but no token counts, and it takes a few milliseconds;
   - its machines use different models (`gpt-6.1-sol` at `low` and `high`, and `gpt-6-astra` for one reviewer), in each `usage` event;
   - some `text` arrives a few seconds into a turn (a reviewer's first sentence), but most arrives when the machine finishes.

   Keep its graph and record in the reference repo as demo material, under `demo/implement-with-3-big-brains/`, laid out as a real target is (`.assembly-lines/implement-with-3-big-brains/assembly-line.dot` and `.assembly-lines/implement-with-3-big-brains/runs/tetris-007/record.jsonl`), so the command finds the graph without help. An earlier, partly finished copy of the same run (its first 201 lines) was superseded by this one.
3. A `three-big-brains` run, if one exists by then. It's not required.

Both must look good at a laptop's screen size, shared on a call.

## Constraints

- **One parser.** Read the graph with the factory's existing line parsing (`line.go`). Find the planner, the route's order and the back edges the way `watch`'s route does, reusing its code.
- **Finding the graph.** A record lives at `<target>/.assembly-lines/<line>/runs/<run>/record.jsonl`, so the graph is `../../assembly-line.dot` from it. The run's name and line come from that path, or from `run.json` beside the record when it's there.
- **No network.** The page works offline: no CDN, no fonts or scripts from the web. Everything it needs, including the kit's CSS, its wobble filters and the Patrick Hand, Patrick Hand SC and Luckiest Guy fonts (all under the SIL Open Font License, licence files in the brand repo), is built into the binary.
- **No new tools for students.** It mustn't need Graphviz or Node on the machine it runs on. The release stays one static binary per platform.
- **It depends only on the record and the graph,** never on the plan or on any machine's prompt or result fields, as `watch` does.
- **VS Code's Simple Browser, in a Codespace in the browser** (David, 2026-10-26: that's where he demos). The page must work there as well as in an ordinary browser tab:
  - Simple Browser shows the page in a frame, at the Codespace's forwarded address (`https://<codespace>-<port>.app.github.dev`), not at `localhost`. So the page uses relative addresses only, and the server answers whatever host name it's reached by. It doesn't check the `Host` or `Origin` header.
  - It often sits in a narrow editor pane. The drawing scales to the pane's width, the bar wraps rather than overflowing, and the bubble is kept inside the pane (the frame's own window), as it is kept inside a browser window.
  - Nothing that stops the page being shown in a frame (David, 2026-10-26). The server sends no `X-Frame-Options` header and no `Content-Security-Policy` with `frame-ancestors` (not even `'self'`, since Simple Browser's frame is on another origin), and the page has no `<meta http-equiv>` directive doing the same, and no script that breaks out of a frame or refuses to run in one. A test checks every response's headers for these.
  - Nothing a frame might block: no pop-up windows, no `alert` or `confirm`, nothing from the network, and no storage it can't work without.
  - Every control works with a click. VS Code takes many keys for itself, so keyboard shortcuts are a bonus and never the only way to do something.
  - The command prints the address as `http://localhost:<port>/`, which VS Code turns into a forwarded link, and a line saying how to open it in Simple Browser: Codespaces offers "Preview in Editor" when the port is forwarded, or the command palette's "Simple Browser: Show".
  - A headless browser test loads the page inside a narrow frame on another origin and checks that it draws, plays and opens a bubble. The real check is by hand: David opens the demo run in Simple Browser in a Codespace in the browser, half the screen wide, and plays it to the end.
- **Accessibility.** Colour never carries meaning alone: done machines have a tick, working machines say `working` in their bubble, and back edges have their labels.

## Non-goals

- A live view of a running run. `watch` does that.
- Steering, or anything that changes a run.
- Showing the plan, prompts or the code the run made, or results beyond the short line in the bubble.
- **Tool calls** (David, 2026-10-26: skip for now). The bridge already sends them, and the factory drops them (`internal/machine/acp.go`). Recording them as a new kind of event would let the bubble fill while a machine works. It needs a new release and a new demo run, so it's a follow-up.
- Changes to the starter, the tutorial or the homeworks. The students' own factory doesn't get this command.

## Calls I made (not yet confirmed by David)

- The command's name is `visualise-run`, with British spelling, as in David's wording.
- It takes the record's path. A `--graph <path>` flag overrides where the graph is found, for a record kept somewhere else.
- It listens on a free port on `localhost` by default, with `--port` to choose one, prints the address, and doesn't try to open a browser. It keeps serving until Ctrl-C.
- The demo material goes in `demo/implement-with-3-big-brains/` in the reference repo.
- 10× is the starting speed.
- Which edge the work took isn't in the record. It's worked out from the record: when a machine starts, the edge it came along is the one from the machine that most recently ended among those with an edge to it. For a machine after a fan out (the synthesiser), each branch's edge counts once.
- `finish` lights up when the record's last event is the run finishing.
- **Layout of the demo line.** The planning part reads first, then the building loop: `start`, `plan_gate`, `plan_author`, the three plan reviewers stacked, `plan_synthesiser`, `doer`, `validator`, `planner`, `finish`, left to right. The edges back are then `not satisfied` (to `plan_author`), `retry` (to `doer`), `replan` (to `plan_author`) and `not complete` (to `doer`). A layout that breaks cycles by following edges in the order they're written would put `doer` right after `plan_gate`, because `pending` is written first, so the builder can't rely on that alone.
- **Blue.** Every edge that goes back is blue, as the kit's loop-back arrow is. On the timeline, only a **send-back** gets a blue tick: work going back along a back edge where neither end is the planner. On the demo line those are `not satisfied`, `retry` and `replan`; on three-big-brains, `not satisfied`; ralph-loop has none, since `ralph → planner` is its normal loop. The planner's `not complete` loop isn't one either: the planner is at one end of it. Other hand-offs get slate ticks. (`canSendWorkBack` in `line.go` isn't enough: `retry` and `replan` aren't `not …` labels.)
- **A record holding a stop and a resume** (like `tetris/release-ride`'s and David's demo run, with more than one `run started` event) replays as one run:
  - the idle gap between the stop and the restart is cut out of the timeline, and marked there with a small break;
  - the elapsed time and the total count only the time the run was going;
  - the replay shows the stop as it passes it, under the drawing ("stopped: <reason>, started again");
  - rounds carry on counting across the resume;
  - the first machine after a restart came from `start`, so that's the edge it lights.
- **`replan` and `not satisfied` have no real record.** David's demo run has two `retry` send-backs but neither of those, so the builder makes a small stand-in record by hand on the demo line for tests of all three. David's real record is the acceptance check for everything else. Nobody makes a real run for this work.
- **Releasing** isn't part of this work. David publishes a release with `bin/publish-release` when he's ready.

## Left to the builder

- How the graph is laid out, as long as it reads left to right, is stable for a given line, handles fan out and back edges cleanly for both lines above, and needs no external tool. For example, a layered layout worked out in Go or in the page.
- How the page gets the run: one JSON document built by the command, or the record served as it is.
- The page's code: plain HTML, CSS and JavaScript built into the binary with `embed`, or anything else that needs no build step for students.
- How the wobble is drawn, as long as it matches the kit.
- Tests: the replay's state at a moment in time (working, done, round, edge counts, tokens) is the part worth testing hard, from real records.

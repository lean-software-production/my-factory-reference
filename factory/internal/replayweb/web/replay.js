// The replay page. It reads the replay the factory worked out (run.json) and
// draws the run's assembly line as it stood at a moment of the run. The Go
// side decides everything that happened; this only draws it. Names and
// machine text are always set as text, never as HTML.
'use strict';

const SVG_NS = 'http://www.w3.org/2000/svg';

// What the page shows: the replay, the moment, and how it plays.
const state = { doc: null, t: 0, playing: false, speed: 10 };

// Sizes, in the drawing's own units, before it is scaled to the page.
const SIZE = {
  boxH: 76, boxPad: 12, boxMinW: 96,     // machines, with a name on one line
  nameLineH: 24,                         // each further line of a machine's name
  wrapAt: 90,                            // a name wider than this goes on two lines
  pillH: 50, pillPad: 16, pillMinW: 70,  // start and finish
  rowGap: 44,                            // between the tallest boxes of two rows
  gapMin: 44,                            // between columns
  portSide: 12,                          // how far a curving edge leaves a box's side off its middle
  portStep: 22,                          // between edges meeting a box's top or bottom
  arcBase: 44, arcStep: 28,              // how far an arc clears the boxes, and how much more per column
  labelH: 26, labelPad: 3,
  badgeR: 14,
  margin: 10,                            // round the drawing, for the wobble
  maxScale: 1.4,                         // the most the drawing grows past its own size
};

// The parts of the drawing that show() changes, made by draw().
let view = null;

// ---------- times, as watch shows them ----------

// fmtElapsed is a time gone, to the second: 40s, 6m 40s, 1h 5m 3s.
function fmtElapsed(seconds) {
  const s = Math.max(0, Math.floor(seconds));
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m ${s % 60}s`;
  return `${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m ${s % 60}s`;
}

// fmtStamp is a log line's time into the run: 6:31, or 1:02:03 from an hour.
function fmtStamp(seconds) {
  const s = Math.max(0, Math.floor(seconds));
  const ss = String(s % 60).padStart(2, '0');
  if (s < 3600) return `${Math.floor(s / 60)}:${ss}`;
  const mm = String(Math.floor((s % 3600) / 60)).padStart(2, '0');
  return `${Math.floor(s / 3600)}:${mm}:${ss}`;
}

// ---------- frames ----------

// frameAt is the last frame at or before t, found by halving.
function frameAt(frames, t) {
  if (!frames || frames.length === 0) return null;
  let lo = 0, hi = frames.length - 1;
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if (frames[mid].at <= t) lo = mid; else hi = mid - 1;
  }
  return frames[lo];
}

// ---------- SVG helpers ----------

// svgEl makes an SVG element with the given attributes, in parent.
function svgEl(tag, attrs, parent) {
  const e = document.createElementNS(SVG_NS, tag);
  for (const [k, v] of Object.entries(attrs || {})) e.setAttribute(k, v);
  if (parent) parent.appendChild(e);
  return e;
}

// svgText is a line of text centred on x, its words set as text.
function svgText(parent, cls, words, x, y) {
  const t = svgEl('text', { class: cls, x, y, 'text-anchor': 'middle' }, parent);
  t.textContent = words;
  return t;
}

// textWidth measures words as the drawing shows them in class cls.
function textWidth(svg, cls, words) {
  const t = svgText(svg, cls, words, 0, 0);
  const w = t.getComputedTextLength();
  t.remove();
  return w || words.length * 10; // not laid out, as in a hidden frame: a fair guess
}

// cubicMid is the middle (t = 0.5) of a cubic Bézier curve.
function cubicMid(p0, p1, p2, p3) {
  return {
    x: (p0.x + 3 * p1.x + 3 * p2.x + p3.x) / 8,
    y: (p0.y + 3 * p1.y + 3 * p2.y + p3.y) / 8,
  };
}

// ---------- layout: where the boxes go ----------

// modelWidths is how wide the widest model line each machine ever shows
// is drawn, so that its box needn't grow while the run plays.
function modelWidths(svg, frames) {
  const measured = new Map(), widest = {};
  for (const f of frames) {
    for (const [m, model] of Object.entries(f.models || {})) {
      if (!measured.has(model)) measured.set(model, textWidth(svg, 'model', model));
      widest[m] = Math.max(widest[m] || 0, measured.get(model));
    }
  }
  return widest;
}

// nameLines is a machine's name as it is drawn: on one line, or, when that
// is wide, split at the space nearest its middle. The width leaves room
// for a ✓ after the last line.
function nameLines(svg, label) {
  const one = { lines: [label], w: textWidth(svg, 'name', label + ' ✓') };
  if (one.w <= SIZE.wrapAt || !label.includes(' ')) return one;
  let cut = -1;
  for (let i = 0; i < label.length; i++) {
    if (label[i] === ' ' && (cut < 0 || Math.abs(i - label.length / 2) < Math.abs(cut - label.length / 2))) cut = i;
  }
  const lines = [label.slice(0, cut), label.slice(cut + 1)];
  const w = Math.max(textWidth(svg, 'name', lines[0]), textWidth(svg, 'name', lines[1] + ' ✓'));
  return w < one.w ? { lines, w } : one;
}

// boxSizes sizes each node's box to its name and its model line. Every
// machine is as tall as the one with the most lines.
function boxSizes(svg, doc) {
  const models = modelWidths(svg, doc.frames);
  const boxes = new Map();
  for (const n of doc.nodes) {
    if (n.kind === 'machine') {
      const name = nameLines(svg, n.label);
      boxes.set(n.name, {
        node: n,
        lines: name.lines,
        w: Math.max(name.w, models[n.name] || 0, SIZE.boxMinW - 2 * SIZE.boxPad) + 2 * SIZE.boxPad,
      });
    } else {
      const w = textWidth(svg, 'pill', n.label) + 2 * SIZE.pillPad;
      boxes.set(n.name, { node: n, lines: [n.label], w: Math.max(w, SIZE.pillMinW), h: SIZE.pillH });
    }
  }
  const lines = Math.max(1, ...[...boxes.values()].filter(b => b.node.kind === 'machine').map(b => b.lines.length));
  for (const b of boxes.values()) if (b.node.kind === 'machine') b.h = SIZE.boxH + (lines - 1) * SIZE.nameLineH;
  return boxes;
}

// columnsOf numbers the ranks in use 0, 1, 2… left to right, and gives each
// box its column.
function columnsOf(boxes) {
  const ranks = [...new Set([...boxes.values()].map(b => b.node.rank))].sort((a, b) => a - b);
  for (const b of boxes.values()) b.col = ranks.indexOf(b.node.rank);
  return ranks.length;
}

// gapsBetween is how wide each gap between columns is: wide enough for the
// label of every edge drawn straight across it.
function gapsBetween(svg, doc, boxes, columns) {
  const gaps = new Array(Math.max(columns - 1, 0)).fill(SIZE.gapMin);
  for (const e of doc.edges) {
    const a = boxes.get(e.from), b = boxes.get(e.to);
    if (!a || !b || e.back || b.col - a.col !== 1 || !e.label) continue;
    const need = textWidth(svg, 'elabel', e.label) + 2 * SIZE.labelPad + 4;
    gaps[a.col] = Math.max(gaps[a.col], need);
  }
  return gaps;
}

// layout places every box: columns by rank, left to right; rows by row, top
// to bottom, far enough apart for the boxes; each box in the middle of its
// column. Machines in one column share the widest width.
function layout(svg, doc) {
  const boxes = boxSizes(svg, doc);
  const columns = columnsOf(boxes);
  const widths = new Array(columns).fill(0);
  for (const b of boxes.values()) widths[b.col] = Math.max(widths[b.col], b.w);
  for (const b of boxes.values()) if (b.node.kind === 'machine') b.w = widths[b.col];
  const gaps = gapsBetween(svg, doc, boxes, columns);
  const lefts = [0];
  for (let c = 1; c < columns; c++) lefts[c] = lefts[c - 1] + widths[c - 1] + gaps[c - 1];
  const rowStep = Math.max(...[...boxes.values()].map(b => b.h)) + SIZE.rowGap;
  for (const b of boxes.values()) moveBox(b, lefts[b.col] + widths[b.col] / 2, b.node.row * rowStep);
  return boxes;
}

// moveBox puts a box's middle at (cx, cy).
function moveBox(b, cx, cy) {
  Object.assign(b, { cx, cy, x: cx - b.w / 2, y: cy - b.h / 2 });
}

// ---------- routes: how the edges go ----------

// routeKind is how an edge is drawn: straight across to the next column
// ("side"), an arc over the top skipping columns ("over"), an arc underneath
// going back ("under"), or a loop back into its own machine ("self").
function routeKind(e, a, b) {
  if (e.from === e.to) return 'self';
  const span = b.col - a.col;
  if (e.back || span <= 0) return 'under';
  return span === 1 ? 'side' : 'over';
}

// assignPorts spreads the arcs meeting a box's top (or bottom) along it, in
// an order that keeps nested arcs from crossing: those to the left first,
// and of those the nearest first; then those to the right, the farthest
// first.
function assignPorts(routes) {
  const ends = new Map();
  const add = (box, side, route, end, otherX) => {
    const key = box.node.name + ' ' + side;
    if (!ends.has(key)) ends.set(key, { box, list: [] });
    ends.get(key).list.push({ route, end, otherX });
  };
  for (const r of routes) {
    if (!r || (r.kind !== 'over' && r.kind !== 'under')) continue;
    const side = r.kind === 'over' ? 'top' : 'bottom';
    add(r.a, side, r, 'fromX', r.b.cx);
    add(r.b, side, r, 'toX', r.a.cx);
  }
  for (const { box, list } of ends.values()) {
    list.sort((p, q) => (p.otherX >= box.cx) - (q.otherX >= box.cx) || q.otherX - p.otherX);
    const step = list.length > 1 ? Math.min(SIZE.portStep, (box.w - 30) / (list.length - 1)) : 0;
    list.forEach((p, i) => { p.route[p.end] = box.cx + (i - (list.length - 1) / 2) * step; });
  }
}

// between is the boxes in the columns from a's to b's, both included.
function between(boxes, a, b) {
  const lo = Math.min(a.col, b.col), hi = Math.max(a.col, b.col);
  return [...boxes.values()].filter(x => x.col >= lo && x.col <= hi);
}

// sidePath is an edge to the next column: straight along a row, or a gentle
// S between rows, from the right of one box to the left of the next.
function sidePath(r) {
  const { a, b } = r;
  const p0 = { x: a.x + a.w, y: a.cy + Math.sign(b.cy - a.cy) * SIZE.portSide };
  const p3 = { x: b.x - 1, y: b.cy + Math.sign(a.cy - b.cy) * SIZE.portSide };
  if (p0.y === p3.y) return { d: `M${p0.x} ${p0.y} L${p3.x} ${p3.y}`, mid: { x: (p0.x + p3.x) / 2, y: p0.y } };
  const k = (p3.x - p0.x) / 2;
  const p1 = { x: p0.x + k, y: p0.y }, p2 = { x: p3.x - k, y: p3.y };
  return { d: curve(p0, p1, p2, p3), mid: cubicMid(p0, p1, p2, p3) };
}

// arcPath is an arc from the top of one box to the top of another, over the
// boxes between ("over"), or the same underneath ("under"). The longer the
// span, the farther it goes, so that arcs inside one another stay apart.
function arcPath(r, boxes) {
  const { a, b, kind } = r;
  const span = Math.abs(b.col - a.col);
  const over = kind === 'over';
  const p0 = { x: r.fromX, y: over ? a.y : a.y + a.h };
  const p3 = { x: r.toX, y: over ? b.y - 1 : b.y + b.h + 1 };
  const passed = between(boxes, a, b);
  const apex = over
    ? Math.min(...passed.map(x => x.y)) - SIZE.arcBase - SIZE.arcStep * Math.max(span - 2, 0)
    : Math.max(...passed.map(x => x.y + x.h)) + SIZE.arcBase + SIZE.arcStep * Math.max(span - 1, 0);
  const c = (apex - (p0.y + p3.y) / 8) / 0.75; // control height that puts the curve's middle at apex
  const dx = p3.x - p0.x;
  const k = Math.sign(dx) * Math.min(Math.abs(dx) * 0.05, 24); // leave and arrive nearly upright
  const p1 = { x: p0.x + k, y: c }, p2 = { x: p3.x - k, y: c };
  return { d: curve(p0, p1, p2, p3), mid: cubicMid(p0, p1, p2, p3) };
}

// selfPath is a loop under a box, back into it.
function selfPath(r) {
  const { a } = r, y = a.y + a.h, drop = SIZE.arcBase + 10;
  const p0 = { x: a.cx + 20, y }, p3 = { x: a.cx - 20, y: y + 1 };
  const p1 = { x: a.cx + 50, y: y + drop }, p2 = { x: a.cx - 50, y: y + drop };
  return { d: curve(p0, p1, p2, p3), mid: cubicMid(p0, p1, p2, p3) };
}

// curve is a cubic Bézier as path data.
function curve(p0, p1, p2, p3) {
  const f = p => `${Math.round(p.x * 10) / 10} ${Math.round(p.y * 10) / 10}`;
  return `M${f(p0)} C${f(p1)}, ${f(p2)}, ${f(p3)}`;
}

// routesFor works out each edge's path and the middle where its label goes,
// one per edge in the replay's order. An edge to a node not drawn is null.
function routesFor(doc, boxes, labelWidths) {
  const routes = doc.edges.map((e, i) => {
    const a = boxes.get(e.from), b = boxes.get(e.to);
    if (!a || !b) return null;
    return { edge: e, a, b, kind: routeKind(e, a, b), labelW: labelWidths[i] };
  });
  assignPorts(routes);
  for (const r of routes) {
    if (!r) continue;
    const path = r.kind === 'side' ? sidePath(r) : r.kind === 'self' ? selfPath(r) : arcPath(r, boxes);
    Object.assign(r, path);
  }
  return routes;
}

// labelPlace is where an edge's label and count badge go. Across a gap,
// the label sits just above the line and the badge on it, nudged back off
// the arrowhead; on an arc, both sit on the line, the badge after the label.
function labelPlace(r) {
  const { x, y } = r.mid;
  const half = r.labelW ? r.labelW / 2 + SIZE.labelPad : 0;
  if (r.kind === 'side') {
    const ly = r.labelW ? y - SIZE.badgeR - SIZE.labelH / 2 : y;
    return { lx: x, ly, bx: x - 5, by: y, half };
  }
  return { lx: x, ly: y, bx: r.labelW ? x + half + SIZE.badgeR - 2 : x, by: y, half };
}

// labelBox is the space an edge's label and count take.
function labelBox(r) {
  const p = labelPlace(r), h = SIZE.labelH / 2, b = SIZE.badgeR;
  return {
    x0: Math.min(p.lx - p.half, p.bx - b), x1: Math.max(p.lx + p.half, p.bx + b),
    y0: Math.min(p.ly - h, p.by - b), y1: Math.max(p.ly + h, p.by + b),
  };
}

// bounds is the space the boxes and labels take.
function bounds(boxes, routes) {
  const b = { x0: Infinity, y0: Infinity, x1: -Infinity, y1: -Infinity };
  const grow = (x0, y0, x1, y1) => {
    b.x0 = Math.min(b.x0, x0); b.y0 = Math.min(b.y0, y0);
    b.x1 = Math.max(b.x1, x1); b.y1 = Math.max(b.y1, y1);
  };
  for (const x of boxes.values()) grow(x.x, x.y, x.x + x.w, x.y + x.h);
  for (const r of routes) if (r) { const l = labelBox(r); grow(l.x0, l.y0, l.x1, l.y1); }
  return b;
}

// ---------- drawing ----------

// draw draws the line, with nothing yet working or taken, and keeps the
// parts that show() changes.
function draw(doc) {
  const svg = document.getElementById('drawing');
  svg.replaceChildren();
  const boxes = layout(svg, doc);
  const labelWidths = doc.edges.map(e => (e.label ? textWidth(svg, 'elabel', e.label) : 0));

  // The kit's wobble only reaches 100 units above and left of the origin,
  // so move everything to start just inside it, then route again.
  let routes = routesFor(doc, boxes, labelWidths);
  const first = bounds(boxes, routes);
  for (const b of boxes.values()) moveBox(b, b.cx - first.x0 + SIZE.margin, b.cy - first.y0 + SIZE.margin);
  routes = routesFor(doc, boxes, labelWidths);
  const all = bounds(boxes, routes);
  const width = Math.ceil(all.x1 + SIZE.margin);
  svg.setAttribute('viewBox', `0 0 ${width} ${Math.ceil(all.y1 + SIZE.margin)}`);
  svg.style.maxWidth = `${Math.round(width * SIZE.maxScale)}px`; // a small line doesn't fill a wide page
  svg.setAttribute('aria-label', `the ${doc.line} assembly line`);

  const edgeLayer = svgEl('g', { class: 'edges', filter: 'url(#sk-wobble-line)' }, svg);
  const shapeLayer = svgEl('g', { class: 'shapes', filter: 'url(#sk-wobble-line)' }, svg);
  const wordLayer = svgEl('g', { class: 'words' }, svg);
  const labelLayer = svgEl('g', { class: 'labels' }, svg);

  const nodes = new Map();
  for (const b of boxes.values()) nodes.set(b.node.name, drawNode(b, shapeLayer, wordLayer));
  const edges = routes.map(r => (r ? drawEdge(r, edgeLayer, labelLayer) : null));
  return { nodes, edges, round: document.getElementById('round') };
}

// drawNode draws a machine's box, or the start or finish pill, and its words.
function drawNode(b, shapes, words) {
  const n = b.node, machine = n.kind === 'machine';
  const g = svgEl('g', { class: `node ${n.kind}`, 'data-name': n.name }, shapes);
  svgEl('rect', { x: b.x, y: b.y, width: b.w, height: b.h, rx: machine ? 14 : b.h / 2 }, g);
  const part = { node: n, g, name: null, lastLine: '', model: null };
  if (machine) {
    // The name's lines, then the model line, in the middle of the box.
    let y = b.cy - ((b.lines.length - 1) * SIZE.nameLineH + 27) / 2 + 10.5;
    b.lines.forEach((line, i) => {
      part.name = svgText(words, 'name', line, b.cx, y);
      if (i < b.lines.length - 1) y += SIZE.nameLineH;
    });
    part.lastLine = b.lines[b.lines.length - 1];
    part.model = svgText(words, 'model', '', b.cx, y + 27);
    g.addEventListener('click', () => machineClicked(n.name, g));
  } else {
    part.name = svgText(words, `pill ${n.kind}`, n.label, b.cx, b.cy + 6);
  }
  return part;
}

// drawEdge draws an edge with its arrowhead, its label on a paper backing,
// and its count badge, hidden until the work has gone that way.
function drawEdge(r, edges, labels) {
  const back = r.edge.back ? ' back' : '';
  const path = svgEl('path', { class: `edge quiet${back}`, d: r.d, 'marker-end': 'url(#sk-head)' }, edges);
  path.dataset.from = r.edge.from;
  path.dataset.to = r.edge.to;
  const p = labelPlace(r);
  if (r.edge.label) {
    svgEl('rect', { class: 'elabel-bg', x: p.lx - p.half, y: p.ly - SIZE.labelH / 2, width: 2 * p.half, height: SIZE.labelH, rx: 8 }, labels);
    svgText(labels, 'elabel', r.edge.label, p.lx, p.ly + 6);
  }
  const badge = svgEl('g', { class: `badge${back}` }, labels);
  svgEl('circle', { cx: p.bx, cy: p.by, r: SIZE.badgeR }, badge);
  const count = svgText(badge, 'count', '', p.bx, p.by + 6);
  return { path, badge, count };
}

// ---------- the moment shown ----------

// show draws the run as it stood t seconds in: who is working, who is done
// this round, the models, the edges taken and how often, and the round.
function show(t) {
  const doc = state.doc;
  if (!doc || !view) return;
  state.t = Math.min(Math.max(t, 0), doc.total || 0);
  const f = frameAt(doc.frames, state.t);
  if (!f) return;
  const working = new Set(f.working), done = new Set(f.done);
  for (const [name, part] of view.nodes) {
    if (part.node.kind === 'machine') {
      part.g.classList.toggle('working', working.has(name));
      part.g.classList.toggle('done', done.has(name));
      part.name.textContent = part.lastLine + (done.has(name) ? ' ✓' : '');
      part.model.textContent = (f.models && f.models[name]) || '';
    } else if (part.node.kind === 'finish') {
      part.g.classList.toggle('lit', f.finished);
      part.name.classList.toggle('lit', f.finished);
    }
  }
  view.edges.forEach((e, i) => {
    if (!e) return;
    const n = (f.counts && f.counts[i]) || 0;
    e.path.classList.toggle('quiet', n === 0);
    e.badge.classList.toggle('shown', n > 0);
    e.count.textContent = String(n);
  });
  view.round.textContent = `round ${f.round}`;
  showBar(f);
  showNote();
  showLog(f);
}

// ---------- playing ----------

// How the run plays, in run seconds unless named otherwise.
const PLAY = {
  lead: 1,          // next hand-off stops this long before the machine starts
  together: 0.25,   // hand-offs closer than this to t are taken as reached
  breakShown: 5,    // a stop's line shows for this long after it
  breakHeldMs: 3000, // and, when playing passes it, for at least this long in real time
  maxStepMs: 250,   // the most real time one frame of playing counts, as after a hidden tab
};

let lastTick = 0;
let frameAsked = 0; // the frame playing has asked for, so that pause can cancel it

// play plays from t, or from the start again if t is the end.
function play() {
  const doc = state.doc;
  if (!doc || state.playing) return;
  if (state.t >= doc.total) jump(0);
  state.playing = true;
  lastTick = performance.now();
  showPlaying();
  frameAsked = requestAnimationFrame(tick);
}

function pause() {
  state.playing = false;
  cancelAnimationFrame(frameAsked);
  showPlaying();
}

// tick moves t on by the real time since the last frame, times the speed,
// and stops at the end.
function tick(now) {
  if (!state.playing) return;
  const gone = Math.min(Math.max(now - lastTick, 0), PLAY.maxStepMs) / 1000;
  lastTick = now;
  const from = state.t, to = Math.min(from + gone * state.speed, state.doc.total);
  for (const b of state.doc.breaks || []) if (from < b.at && b.at <= to) holdBreak(b);
  show(to);
  if (state.t >= state.doc.total) pause();
  else frameAsked = requestAnimationFrame(tick);
}

// jump shows t, from a click on the bar rather than from playing.
function jump(t) {
  heldBreak = null;
  show(t);
}

// showPlaying shows play or pause on the button, whichever a click does.
function showPlaying() {
  const b = document.getElementById('play');
  const does = state.playing ? 'pause' : 'play';
  b.textContent = state.playing ? '⏸︎' : '▶︎';
  b.setAttribute('aria-label', does);
  b.title = does;
}

function setSpeed(speed) {
  state.speed = speed;
  for (const b of document.querySelectorAll('.speed')) {
    const on = Number(b.dataset.speed) === speed;
    b.classList.toggle('on', on);
    b.setAttribute('aria-pressed', String(on));
  }
}

// nextHandoff jumps to just before the next hand-off: the first whose
// moment, a second early, is after t. Hand-offs a moment apart count as one.
function nextHandoff() {
  const t = state.t;
  const h = (state.doc.handoffs || []).find(h => Math.max(h.at - PLAY.lead, 0) > t + PLAY.together);
  if (h) jump(h.at - PLAY.lead);
}

// ---------- the bar ----------

// buildTimeline puts a tick on the timeline for each hand-off, blue for a
// send-back, and a break where the run stopped and started again, its
// reason as the tooltip.
function buildTimeline(doc) {
  const track = document.getElementById('track');
  const at = t => `${(doc.total > 0 ? Math.min(Math.max(t / doc.total, 0), 1) : 0) * 100}%`;
  for (const h of doc.handoffs || []) {
    const i = document.createElement('i');
    if (h.sendBack) i.className = 'back';
    i.style.left = at(h.at);
    track.appendChild(i);
  }
  for (const b of doc.breaks || []) {
    const s = document.createElement('s');
    s.style.left = at(b.at);
    s.title = b.reason;
    track.appendChild(s);
  }
}

// wireBar makes the bar's controls work. Every one works with a click; the
// space bar playing and pausing is only a shortcut.
function wireBar() {
  document.getElementById('play').addEventListener('click', () => (state.playing ? pause() : play()));
  for (const b of document.querySelectorAll('.speed')) b.addEventListener('click', () => setSpeed(Number(b.dataset.speed)));
  document.getElementById('next').addEventListener('click', nextHandoff);

  const line = document.getElementById('timeline');
  const seek = e => {
    const r = document.getElementById('track').getBoundingClientRect();
    const part = r.width > 0 ? Math.min(Math.max((e.clientX - r.left) / r.width, 0), 1) : 0;
    jump(part * state.doc.total);
  };
  line.addEventListener('pointerdown', e => {
    if (e.button !== 0) return;
    e.preventDefault();
    line.setPointerCapture(e.pointerId);
    seek(e);
  });
  line.addEventListener('pointermove', e => { if (line.hasPointerCapture(e.pointerId)) seek(e); });

  document.addEventListener('keydown', e => {
    if (e.key !== ' ' || e.repeat) return;
    if (e.target instanceof Element && e.target.closest('button, input, select, textarea, [contenteditable]')) return;
    e.preventDefault();
    if (state.playing) pause(); else play();
  });
}

// steadyMeta keeps the time and tokens as wide as they will ever be, at
// the end, so that the timeline beside them doesn't twitch as they change.
function steadyMeta(doc) {
  const meta = document.getElementById('meta');
  meta.textContent = `${fmtElapsed(doc.total)} of ${fmtElapsed(doc.total)} · ${(doc.end && doc.end.tokens) || '888.8k'} tokens`;
  meta.style.minWidth = `${Math.ceil(meta.getBoundingClientRect().width) + 12}px`;
}

// showBar fills the timeline to t and says how far in the run is.
function showBar(f) {
  const total = state.doc.total || 0;
  document.getElementById('fill').style.width = `${total > 0 ? (state.t / total) * 100 : 0}%`;
  document.getElementById('meta').textContent = `${fmtElapsed(state.t)} of ${fmtElapsed(total)} · ${f.tokens} tokens`;
}

// ---------- the line under the drawing ----------

// heldBreak is a stop that playing has just passed, kept on show for a
// moment of real time: at 60× its five run seconds go by in a blink.
let heldBreak = null;

function holdBreak(b) {
  heldBreak = { reason: b.reason, until: performance.now() + PLAY.breakHeldMs };
  setTimeout(showNote, PLAY.breakHeldMs + 50);
}

// showNote says, under the drawing, that the run stopped and started again
// just here, or how it ended once t is the end.
function showNote() {
  const doc = state.doc, t = state.t;
  const near = (doc.breaks || []).filter(b => t >= b.at && t - b.at <= PLAY.breakShown).pop();
  const held = heldBreak && performance.now() < heldBreak.until ? heldBreak : null;
  let words = '';
  if (near || held) words = `Stopped here, and started again: ${(near || held).reason}`;
  else if (t >= doc.total) words = endWords(doc);
  document.getElementById('note').textContent = words;
}

// endWords is how the run ended.
function endWords(doc) {
  const e = doc.end || {}, when = fmtElapsed(doc.total);
  if (e.state === 'finished') return `The run finished in ${when}, using ${e.tokens} tokens.`;
  if (e.state === 'stopped' || e.state === 'failed') return `The run ${e.state} at ${when}: ${e.reason}`;
  return `The record ends at ${when}, before the run finished.`;
}

// ---------- the bubble: a machine's log ----------

// Each machine's lines, in time order, and the bubble open, if one is.
let linesByMachine = new Map();
let openLog = null; // { name, label, rect, shown: lines shown, heading }

// Where the bubble goes, in CSS pixels: how far from the machine, how wide
// its tail is, and how close it may come to the window's edge.
const BUBBLE = { gap: 16, tail: 19, edge: 8, corner: 14 };

function groupLines(lines) {
  const by = new Map();
  for (const l of lines || []) {
    if (!by.has(l.machine)) by.set(l.machine, []);
    by.get(l.machine).push(l);
  }
  return by;
}

// countUpTo is how many of lines, in time order, are at or before t.
function countUpTo(lines, t) {
  let lo = 0, hi = lines.length;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    if (lines[mid].at <= t) lo = mid + 1; else hi = mid;
  }
  return lo;
}

function wireLog() {
  document.getElementById('logclose').addEventListener('click', closeLog);
  addEventListener('resize', placeLog);
  addEventListener('scroll', placeLog, { passive: true });
}

// machineClicked opens a machine's log, closes it if it is open, or moves
// the bubble to it from another machine.
function machineClicked(name, shape) {
  if (openLog && openLog.name === name) {
    closeLog();
    return;
  }
  const part = view.nodes.get(name);
  openLog = { name, label: part.node.label, rect: shape.querySelector('rect'), shown: -1, heading: '' };
  document.getElementById('log').hidden = false;
  showLog(frameAt(state.doc.frames, state.t));
}

function closeLog() {
  openLog = null;
  document.getElementById('log').hidden = true;
}

// showLog brings the open bubble up to t: working or not, and the lines
// said by then, the newest last and highlighted, scrolled to the bottom.
function showLog(f) {
  if (!openLog || !f) return;
  const heading = `${openLog.label} · ${f.working.includes(openLog.name) ? 'working' : 'last time'}`;
  const lines = linesByMachine.get(openLog.name) || [];
  const n = countUpTo(lines, state.t);
  if (heading === openLog.heading && n === openLog.shown) return;
  document.getElementById('logname').textContent = heading;
  if (n !== openLog.shown) {
    const box = document.getElementById('loglines');
    if (n === 0) {
      const p = document.createElement('p');
      p.className = 'none';
      p.textContent = 'nothing yet';
      box.replaceChildren(p);
    } else {
      box.replaceChildren(...lines.slice(0, n).map((l, i) => logLine(l, i === n - 1)));
    }
    box.scrollTop = box.scrollHeight;
  }
  Object.assign(openLog, { heading, shown: n });
  placeLog();
}

// logLine is one line of the log: its stamp, then what was said, or the
// result as a short line after an arrow. All of it set as text.
function logLine(l, newest) {
  const p = document.createElement('p');
  if (newest) p.className = 'now';
  const stamp = document.createElement('span');
  stamp.className = 't';
  stamp.textContent = fmtStamp(l.at);
  p.appendChild(stamp);
  if (l.result) {
    const res = document.createElement('span');
    res.className = 'res';
    res.textContent = `→ ${l.text}`;
    p.appendChild(res);
  } else {
    p.appendChild(document.createTextNode(l.text));
  }
  return p;
}

// placeLog puts the bubble above its machine, the tail pointing down at the
// machine's middle. If there isn't room above in the window, it goes below
// with the tail pointing up; if there's room on neither side, on the side
// with more. It is kept inside the window, and if that pulls the tail off
// the machine, the tail is hidden.
function placeLog() {
  if (!openLog) return;
  const log = document.getElementById('log');
  const r = openLog.rect.getBoundingClientRect();
  const s = document.getElementById('stage').getBoundingClientRect();
  const winW = document.documentElement.clientWidth, winH = window.innerHeight;
  const w = log.offsetWidth, h = log.offsetHeight;
  const { gap, tail, edge, corner } = BUBBLE;
  const roomAbove = r.top - gap - edge, roomBelow = winH - edge - r.bottom - gap;
  const above = h <= roomAbove || (h > roomBelow && roomAbove >= roomBelow);
  log.classList.toggle('below', !above);
  // Inside the window top to bottom; a bubble taller than the window keeps its heading in view.
  const top = Math.max(Math.min(above ? r.top - h - gap : r.bottom + gap, winH - h - edge), edge);
  const covers = above ? top + h > r.top : top < r.bottom; // over the machine: the tail can't point at it
  const cx = r.left + r.width / 2;
  const left = Math.max(Math.min(cx - w / 2, winW - w - edge), edge);
  const tailX = cx - left - tail / 2;
  log.classList.toggle('no-tail', covers || tailX < corner || tailX > w - corner - tail);
  log.style.setProperty('--tail-x', `${tailX}px`);
  log.style.left = `${left - s.left}px`;
  log.style.top = `${top - s.top}px`;
}

// ---------- starting ----------

// heading names the run above the drawing.
function heading(doc) {
  document.title = `${doc.line} · run ${doc.run}`;
  document.getElementById('title').textContent = `the ${doc.line} line`;
  document.getElementById('where').textContent = `${doc.target} · ${doc.line} · run "${doc.run}"`;
}

// fontsLoaded waits for the kit's fonts, so text is measured in them.
async function fontsLoaded() {
  try {
    await Promise.all(['22px "Patrick Hand SC"', '17px "Patrick Hand"'].map(f => document.fonts.load(f)));
  } catch (err) {
    // drawn in the fallback fonts, then
  }
}

async function boot() {
  const res = await fetch('run.json');
  if (!res.ok) throw new Error(`run.json: ${res.status}`);
  state.doc = await res.json();
  heading(state.doc);
  await fontsLoaded();
  view = draw(state.doc);
  linesByMachine = groupLines(state.doc.lines);
  buildTimeline(state.doc);
  steadyMeta(state.doc);
  wireBar();
  wireLog();
  show(0); // paused at the start, so there's time to talk first
}

boot().catch(err => {
  document.getElementById('where').textContent = `Could not draw the run: ${err.message}`;
});

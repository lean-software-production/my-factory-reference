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
  boxH: 76, boxPad: 20, boxMinW: 110,    // machines
  pillH: 50, pillPad: 24, pillMinW: 80,  // start and finish
  rowStep: 120,                          // from one row's middle to the next
  gapMin: 52,                            // between columns
  portSide: 12,                          // how far a curving edge leaves a box's side off its middle
  portStep: 22,                          // between edges meeting a box's top or bottom
  arcBase: 44, arcStep: 28,              // how far an arc clears the boxes, and how much more per column
  labelH: 24, labelPad: 6,
  badgeR: 12,
  margin: 20,                            // round the drawing, for the wobble
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

// widestModels is the longest model line each machine ever shows, so that
// its box needn't grow while the run plays.
function widestModels(frames) {
  const widest = {};
  for (const f of frames) {
    for (const [m, model] of Object.entries(f.models || {})) {
      if (!widest[m] || model.length > widest[m].length) widest[m] = model;
    }
  }
  return widest;
}

// boxSizes sizes each node's box to its name (with room for a ✓) and its
// model line. Machines in one column share the widest width.
function boxSizes(svg, doc) {
  const models = widestModels(doc.frames);
  const boxes = new Map();
  for (const n of doc.nodes) {
    const machine = n.kind === 'machine';
    let w;
    if (machine) {
      const name = textWidth(svg, 'name', n.label + ' ✓');
      const model = models[n.name] ? textWidth(svg, 'model', models[n.name]) : 0;
      w = Math.max(name, model) + 2 * SIZE.boxPad;
    } else {
      w = textWidth(svg, 'pill', n.label) + 2 * SIZE.pillPad;
    }
    boxes.set(n.name, {
      node: n,
      w: Math.max(w, machine ? SIZE.boxMinW : SIZE.pillMinW),
      h: machine ? SIZE.boxH : SIZE.pillH,
    });
  }
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
    const need = textWidth(svg, 'elabel', e.label) + 2 * SIZE.labelPad + 12;
    gaps[a.col] = Math.max(gaps[a.col], need);
  }
  return gaps;
}

// layout places every box: columns by rank, left to right; rows by row, top
// to bottom; each box in the middle of its column.
function layout(svg, doc) {
  const boxes = boxSizes(svg, doc);
  const columns = columnsOf(boxes);
  const widths = new Array(columns).fill(0);
  for (const b of boxes.values()) widths[b.col] = Math.max(widths[b.col], b.w);
  for (const b of boxes.values()) if (b.node.kind === 'machine') b.w = widths[b.col];
  const gaps = gapsBetween(svg, doc, boxes, columns);
  const lefts = [0];
  for (let c = 1; c < columns; c++) lefts[c] = lefts[c - 1] + widths[c - 1] + gaps[c - 1];
  for (const b of boxes.values()) moveBox(b, lefts[b.col] + widths[b.col] / 2, b.node.row * SIZE.rowStep);
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
// the label sits just above the line and the badge on it; on an arc, both
// sit on the line, the badge after the label.
function labelPlace(r) {
  const { x, y } = r.mid;
  const half = r.labelW ? r.labelW / 2 + SIZE.labelPad : 0;
  if (r.kind === 'side') {
    const ly = r.labelW ? y - SIZE.badgeR - SIZE.labelH / 2 : y;
    return { lx: x, ly, bx: x, by: y, half };
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
  const part = { node: n, g, name: null, model: null };
  if (machine) {
    part.name = svgText(words, 'name', n.label, b.cx, b.cy - 3);
    part.model = svgText(words, 'model', '', b.cx, b.cy + 23);
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
  const count = svgText(badge, 'count', '', p.bx, p.by + 5);
  return { path, badge, count };
}

// machineClicked is where a click on a machine lands. The machine's log, in
// a speech bubble, opens from here; for now it does nothing.
function machineClicked(name, shape) {}

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
      part.name.textContent = part.node.label + (done.has(name) ? ' ✓' : '');
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
  show(570); // for now, the moment the approved mockup shows
}

boot().catch(err => {
  document.getElementById('where').textContent = `Could not draw the run: ${err.message}`;
});

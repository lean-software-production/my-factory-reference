'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const { EventEmitter } = require('node:events');
const path = require('node:path');

const source = fs.readFileSync(path.join(__dirname, '..', 'index.js'), 'utf8');

function game() {
  const input = new EventEmitter();
  const rawModes = [];
  input.isTTY = true;
  input.setRawMode = (mode) => rawModes.push(mode);
  input.resume = () => {};
  input.pause = () => { input.paused = true; };
  const output = { isTTY: true, text: '', write(text) { this.text += text; } };
  const timers = new Map();
  let id = 0;
  let exited = false;
  const proc = new EventEmitter();
  proc.stdin = input;
  proc.stdout = output;
  proc.exit = () => { exited = true; proc.emit('exit'); };
  const context = vm.createContext({
    process: proc,
    require: (name) => name === 'node:readline' ? { emitKeypressEvents() {} } : require(name),
    setTimeout: (fn, delay) => { timers.set(++id, { fn, delay }); return id; },
    clearTimeout: (timer) => timers.delete(timer),
    Math: Object.assign(Object.create(Math), { random: () => 0.2 }), // Always spawn O.
  });
  vm.runInContext(source, context, { filename: 'index.js' });
  return {
    eval: (code) => vm.runInContext(code, context),
    key: (name, ctrl = false) => input.emit('keypress', '', { name, ctrl }),
    screen: () => output.text.split('\x1b[H\x1b[2J').at(-1).trimEnd().split('\n'),
    output, timers, rawModes, input,
    get exited() { return exited; },
  };
}

test('initial screen and game-over screen each fit in 24 rows', () => {
  const g = game();
  assert.equal(g.screen().length, 24);
  assert.match(g.screen()[2], /Score: 0/);
  assert.match(g.screen()[3], /Lines: 0/);
  assert.match(g.screen().join('\n'), /Space: Hard drop/);
  g.eval('activePiece = null; render()');
  assert.equal(g.screen().length, 24);
  assert.match(g.screen().join('\n'), /GAME OVER - Press R to restart, Q to quit/);
  g.key('r');
  assert.equal(g.screen().length, 24);
  assert.match(g.screen()[2], /Score: 0/);
  assert.equal(g.eval('activePiece !== null'), true);
});

test('movement respects walls and settled cells; rotation does not overwrite blocks', () => {
  const g = game();
  g.eval("activePiece = { type: 'T', x: 0, y: 0, cells: SHAPES.T }");
  g.key('left');
  assert.equal(g.eval('activePiece.x'), 0);
  g.eval("settled[1][3] = 'Z'");
  g.key('right');
  assert.equal(g.eval('activePiece.x'), 0);
  assert.equal(g.eval("canPlace(activePiece, 0, 0)"), true);
  assert.equal(g.eval("canPlace(activePiece, 0, 19)"), false);
  g.eval("settled[0][0] = 'J'; settled[0][2] = 'J'; settled[1][0] = ' '; settled[1][3] = ' '");
  g.key('up');
  assert.equal(g.eval('activePiece.cells.length'), 4);
  assert.equal(g.eval('canPlace(activePiece, activePiece.x, activePiece.y)'), true);
  g.eval("settled = Array.from({ length: HEIGHT }, () => Array(WIDTH).fill(' ')); activePiece = { type: 'I', x: 7, y: 0, cells: SHAPES.I }");
  g.key('up');
  g.key('up');
  assert.equal(g.eval('activePiece.x'), 6); // wall kick to keep the horizontal I inside
  assert.equal(g.eval('canPlace(activePiece, activePiece.x, activePiece.y)'), true);
});

test('gravity descends one row and soft drop locks at the floor', () => {
  const g = game();
  const timer = [...g.timers.values()][0];
  assert.equal(timer.delay, 700);
  timer.fn();
  assert.equal(g.eval('activePiece.y'), 1);
  g.eval('activePiece.y = 18');
  g.key('down');
  assert.equal(g.eval('activePiece.y'), 0); // O cannot fall past the floor; a new one spawns
  assert.equal(g.eval("settled[18][4]"), 'O');
});

test('hard drop locks a piece, clears multiple lines, scores and speeds up', () => {
  const g = game();
  g.eval(`activePiece = { type: 'O', x: 3, y: 0, cells: SHAPES.O };
    for (let row of [18, 19]) for (let col = 0; col < WIDTH; col++)
      if (col !== 4 && col !== 5) settled[row][col] = 'I';`);
  g.key('space');
  assert.equal(g.eval('linesCleared'), 2);
  assert.equal(g.eval('score'), 300);
  assert.equal(g.eval("settled.every(row => row.every(cell => cell === ' '))"), true);
  assert.equal(g.eval('activePiece !== null'), true);
  g.eval(`for (let row = 10; row < 18; row++) settled[row].fill('I'); clearLines()`);
  assert.equal(g.eval('linesCleared'), 10);
  assert.equal(g.eval('fallInterval() < 700'), true);
});

test('blocked spawn ends game; quit cancels gravity and restores terminal', () => {
  const g = game();
  // Lock an O at the bottom while a settled cell occupies the next spawn.
  g.eval("activePiece.y = 18; settled[0][4] = 'J'");
  g.key('space');
  assert.equal(g.eval('activePiece'), null);
  assert.equal(g.screen().length, 24);
  assert.match(g.screen().at(-1), /GAME OVER/);
  assert.deepEqual(g.rawModes, [true]);
  g.key('q');
  assert.equal(g.exited, true);
  assert.deepEqual(g.rawModes, [true, false, false]); // quit and exit handler are both safe
  assert.equal(g.input.paused, true);
  assert.equal(g.timers.size, 0);
  assert.match(g.output.text, /\n$/);
});

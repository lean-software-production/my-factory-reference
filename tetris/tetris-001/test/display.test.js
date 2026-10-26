const assert = require('node:assert/strict');
const { EventEmitter } = require('node:events');
const test = require('node:test');
const { TetrisGame, SHAPES } = require('../src/game');
const { renderGame } = require('../src/render');
const { createTerminal, MAX_ROWS } = require('../src/terminal');
const { runGame } = require('../src/index');

const types = Object.keys(SHAPES);
function gameFor(type, options = {}) {
  return new TetrisGame({ random: () => (types.indexOf(type) + 0.5) / types.length, ...options });
}

function mockTerminal() {
  const input = new EventEmitter();
  input.isTTY = true;
  input.isRaw = false;
  input.setRawMode = (raw) => { input.isRaw = raw; };
  input.resume = () => {};
  input.pause = () => {};
  const output = { isTTY: true, writes: [], write(text) { this.writes.push(text); } };
  return { input, output };
}

test('playing and game-over displays include all information within 24 rows', () => {
  const game = gameFor('T');
  const playing = renderGame(game);
  assert.equal(playing.length, MAX_ROWS);
  assert.ok(playing[0].startsWith('+'));
  assert.ok(playing[21].startsWith('+'));
  assert.match(playing[22], /Score: 0  Lines: 0/);
  assert.match(playing[23], /Left\/Right.*Up.*Down.*Space.*Q/);
  assert.ok(playing.every((line) => !line.includes('\n')));

  game.board[1][4] = 'J';
  game.spawn();
  const finished = renderGame(game);
  assert.equal(finished.length, MAX_ROWS);
  assert.match(finished[23], /GAME OVER.*Q/);
});

test('display rejects boards taller than the 24-row budget', () => {
  assert.throws(() => renderGame(gameFor('O', { height: 21 })), /24 terminal rows/);
});

test('terminal rejects too many rows and restores raw mode when stopped', () => {
  const { input, output } = mockTerminal();
  const terminal = createTerminal({ input, output, onKey() {} });
  terminal.start();
  assert.equal(input.isRaw, true);
  assert.throws(() => terminal.draw(Array(25).fill('x')), /at most 24/);
  assert.throws(() => terminal.draw(['first\nsecond']), /single-line/);
  terminal.draw(Array(24).fill('x'));
  assert.match(output.writes.at(-1), /^\x1b\[H\x1b\[J/);
  terminal.stop();
  assert.equal(input.isRaw, false);
  assert.equal(output.writes.at(-1), '\x1b[?25h\x1b[?1049l');
});

test('keyboard controls move, rotate, drop and quit a running game', () => {
  const { input, output } = mockTerminal();
  const game = gameFor('T');
  const session = runGame({ game, input, output, intervalMs: 60000 });
  try {
    assert.equal(input.isRaw, true);
    const startX = game.active.x;
    input.emit('keypress', '', { name: 'left' });
    assert.equal(game.active.x, startX - 1);
    input.emit('keypress', '', { name: 'up' });
    assert.equal(game.active.rotation, 1);
    input.emit('keypress', '', { name: 'down' });
    assert.equal(game.score, 1);
    input.emit('keypress', ' ', { name: 'space' });
    assert.ok(game.board.some((row) => row.some((cell) => cell !== null)));
    assert.ok(output.writes.some((text) => text.includes('Score:')));
    input.emit('keypress', 'q', { name: 'q' });
    assert.equal(input.isRaw, false);
  } finally {
    session.stop();
  }
});

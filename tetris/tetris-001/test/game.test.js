const assert = require('node:assert/strict');
const test = require('node:test');
const { TetrisGame, SHAPES } = require('../src/game');

function gameFor(type, options = {}) {
  const types = Object.keys(SHAPES);
  return new TetrisGame({ random: () => (types.indexOf(type) + 0.5) / types.length, ...options });
}

test('all seven pieces spawn with four distinct blocks inside the board', () => {
  for (const type of Object.keys(SHAPES)) {
    const game = gameFor(type);
    assert.equal(game.active.type, type);
    const blocks = game.activeCells();
    assert.equal(blocks.length, 4);
    assert.equal(new Set(blocks.map(([x, y]) => `${x},${y}`)).size, 4);
    assert.ok(blocks.every(([x, y]) => x >= 0 && x < game.width && y >= 0));
  }
});

test('movement respects walls and occupied cells', () => {
  const game = gameFor('T');
  for (let i = 0; i < 3; i++) assert.equal(game.move(-1), true);
  assert.equal(game.move(-1), false);
  assert.equal(game.active.x, 0);
  game.board[1][3] = 'J';
  assert.equal(game.move(1), false);
  assert.equal(game.active.x, 0);
});

test('rotation changes the blocks and kicks away from the right wall', () => {
  const game = gameFor('I');
  const horizontal = game.activeCells();
  assert.equal(game.rotate(), true);
  assert.notDeepEqual(game.activeCells(), horizontal);
  assert.equal(game.move(4), true);
  assert.equal(game.active.x, 7);
  assert.equal(game.rotate(), true);
  assert.equal(game.active.x, 6);
  assert.ok(game.activeCells().every(([x]) => x < game.width));
});

test('falling locks a piece when blocked and spawns the next piece', () => {
  const game = gameFor('O');
  game.board[2][4] = 'J';
  assert.equal(game.step(), false);
  assert.equal(game.board[0][4], 'O');
  assert.equal(game.board[1][5], 'O');
  assert.equal(game.gameOver, true); // Next O cannot spawn through the locked one.
});

test('hard drop clears a completed row and awards drop and line points', () => {
  const game = gameFor('O', { width: 4, height: 4 });
  game.board[3][0] = 'J';
  game.board[3][3] = 'J';
  assert.equal(game.hardDrop(), 2);
  assert.equal(game.lines, 1);
  assert.equal(game.score, 104);
  assert.deepEqual(game.board[3], [null, 'O', 'O', null]);
  assert.deepEqual(game.board[0], [null, null, null, null]);
});

test('soft drop scores only on movement; blocked spawning ends the game', () => {
  const game = gameFor('T');
  assert.equal(game.softDrop(), true);
  assert.equal(game.score, 1);
  game.board[1][4] = 'J';
  assert.equal(game.spawn(), false);
  assert.equal(game.gameOver, true);
  assert.equal(game.softDrop(), false);
  assert.equal(game.score, 1);
});

test('visible board overlays active blocks without changing locked cells', () => {
  const game = gameFor('O');
  const before = game.board.map((row) => [...row]);
  const display = game.visibleBoard();
  assert.equal(display[0][4], 'O');
  display[0][4] = 'J';
  assert.deepEqual(game.board, before);
});

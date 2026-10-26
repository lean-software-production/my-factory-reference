const test = require('node:test');
const assert = require('node:assert/strict');
const { SHAPES } = require('../src/pieces');
const { createGame, canPlace, move, rotate, clearLines, tick, hardDrop } = require('../src/game');
const { render, WIDTH, HEIGHT } = require('../src/render');

const choose = type => () => Object.keys(SHAPES).indexOf(type) / Object.keys(SHAPES).length;

test('initial board and seven four-cell pieces', () => {
  assert.equal(Object.keys(SHAPES).length, 7);
  for (const type of Object.keys(SHAPES)) {
    const game = createGame(choose(type));
    assert.equal(game.board.length, HEIGHT);
    assert.ok(game.board.every(row => row.length === WIDTH && row.every(cell => !cell)));
    assert.equal(game.piece.type, type);
    assert.equal(game.piece.cells.length, 4);
    assert.ok(canPlace(game.board, game.piece));
  }
});

test('movement and rotation reject walls and occupied cells', () => {
  const game = createGame(choose('T'));
  while (move(game, -1, 0)) {}
  assert.equal(move(game, -1, 0), false);
  assert.equal(rotate(game), true);
  assert.ok(canPlace(game.board, game.piece));
  const before = game.piece;
  game.board[1][0] = 'X';
  assert.equal(rotate(game), false);
  assert.equal(game.piece, before);
  game.board[1][0] = 0;
  assert.equal(move(game, 1, 0), true);
  const next = { ...game.piece, x: game.piece.x + 1 };
  const [dx, dy] = next.cells[0];
  game.board[next.y + dy][next.x + dx] = 'X';
  assert.equal(move(game, 1, 0), false);
  assert.equal(game.piece.x, next.x - 1);
});

test('gravity locks on collision and spawns another piece', () => {
  const game = createGame(choose('O'));
  game.board[4][4] = 'X';
  tick(game, choose('I'));
  tick(game, choose('I'));
  tick(game, choose('I'));
  assert.equal(game.piece.type, 'I');
  assert.equal(game.board[2][4], 'O');
  assert.equal(game.board[3][5], 'O');
  assert.equal(game.board[4][4], 'X');
});

test('completed rows clear and score updates', () => {
  const game = createGame(choose('O'));
  game.board[HEIGHT - 1].fill('X');
  game.board[HEIGHT - 2].fill('X');
  game.board[HEIGHT - 3][0] = 'J';
  assert.equal(clearLines(game), 2);
  assert.equal(game.score, 200);
  assert.equal(game.board[HEIGHT - 1][0], 'J');
  assert.ok(game.board[0].every(cell => !cell));
});

test('hard drop locks immediately; blocked next spawn ends game', () => {
  const game = createGame(choose('I'));
  game.board[0][4] = 'X'; // Occupies the next O spawn, but not the falling I.
  assert.equal(hardDrop(game, choose('O')), true);
  assert.equal(game.gameOver, true);
  assert.equal(game.piece, null);
  assert.equal(game.board[HEIGHT - 1][3], 'I');
  assert.equal(hardDrop(game), false);
  const restarted = createGame(choose('O'));
  assert.equal(restarted.gameOver, false);
  assert.equal(restarted.score, 0);
});

test('complete display including game over fits within 24 terminal rows', () => {
  const game = createGame(choose('T'));
  for (const over of [false, true]) {
    const output = render(game.board, 200, over ? null : game.piece, over);
    const lines = output.split('\n');
    assert.ok(lines.length <= 24);
    assert.equal(lines.length, HEIGHT + 3);
    assert.ok(lines.every(line => !line.includes('\x1b')));
    assert.match(output, /Score: 200/);
    if (over) assert.match(output, /GAME OVER/);
  }
});

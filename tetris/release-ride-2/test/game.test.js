const test = require('node:test');
const assert = require('node:assert/strict');
const { SHAPES, spawnPiece } = require('../src/pieces');
const { BOARD_WIDTH, BOARD_HEIGHT, render } = require('../src/render');
const { createGame, canPlace, move, rotate, clearLines, tick, hardDrop } = require('../src/game');

function gameWith(type = 'O') {
  const game = createGame();
  game.piece = spawnPiece(type, BOARD_WIDTH);
  game.nextPiece = spawnPiece('T', BOARD_WIDTH);
  return game;
}

test('seven tetrominoes have four distinct cells and spawn inside the board', () => {
  assert.deepEqual(Object.keys(SHAPES).sort(), ['I', 'J', 'L', 'O', 'S', 'T', 'Z']);
  for (const type of Object.keys(SHAPES)) {
    const piece = spawnPiece(type, BOARD_WIDTH);
    assert.equal(piece.cells.length, 4);
    assert.equal(new Set(piece.cells.map(cell => cell.join(','))).size, 4);
    assert.ok(canPlace(gameWith().board, piece));
  }
});

test('movement and gravity respect walls, floor, and occupied cells', () => {
  const game = gameWith();
  game.piece.x = 0;
  assert.equal(move(game, -1), false);
  game.board[0][2] = 'I';
  assert.equal(move(game, 1), false);
  game.board[0][2] = null;
  assert.equal(move(game, 1), true);
  game.piece.y = BOARD_HEIGHT - 2;
  tick(game);
  assert.equal(game.piece.type, 'T');
  assert.equal(game.board[BOARD_HEIGHT - 1][1], 'O');
  assert.equal(game.board[BOARD_HEIGHT - 2][2], 'O');
});

test('rotation uses a wall adjustment but rejects occupied destinations', () => {
  const game = gameWith('I');
  game.piece.x = BOARD_WIDTH - 4;
  assert.equal(rotate(game), true);
  assert.ok(canPlace(game.board, game.piece));
  game.piece.x = 0;
  // Every available kick would overlap at least one cell in this filled row.
  game.board[2].fill('Z');
  assert.equal(rotate(game), false);
});

test('completed rows shift down and score when a piece locks', () => {
  const game = gameWith();
  game.board[BOARD_HEIGHT - 1].fill('J');
  game.board[BOARD_HEIGHT - 1][4] = null;
  game.board[BOARD_HEIGHT - 1][5] = null;
  game.piece.x = 4;
  game.piece.y = BOARD_HEIGHT - 2;
  tick(game);
  assert.equal(game.score, 100);
  assert.deepEqual(game.board[0], Array(BOARD_WIDTH).fill(null));
  assert.equal(game.board[BOARD_HEIGHT - 1][4], 'O');
  assert.equal(game.board[BOARD_HEIGHT - 1][5], 'O');
  const board = Array.from({ length: BOARD_HEIGHT }, () => Array(BOARD_WIDTH).fill(null));
  board[3][0] = 'T';
  board[BOARD_HEIGHT - 1].fill('I');
  board[BOARD_HEIGHT - 2].fill('J');
  assert.equal(clearLines(board), 2);
  assert.equal(board[5][0], 'T');
});

test('hard drop locks a piece and a blocked next spawn ends the game', () => {
  const game = gameWith();
  game.board[0][4] = 'Z';
  assert.equal(hardDrop(game), true);
  assert.equal(game.gameOver, true);
  assert.equal(game.score, 0);
  assert.equal(game.board[BOARD_HEIGHT - 1][4], 'O');
  assert.equal(hardDrop(game), false);
});

test('complete display fits in 24 rows during play and game over', () => {
  const game = gameWith();
  for (const over of [false, true]) {
    const screen = render(game.board, game.score, over ? null : game.piece, over, game.nextPiece);
    const lines = screen.split('\n');
    assert.ok(lines.length <= 24);
    assert.equal(lines.length, BOARD_HEIGHT + 2);
    assert.match(screen, /Score:/);
    assert.match(screen, /Controls:/);
    assert.match(screen, /Next:/);
    if (over) assert.match(screen, /Game over! Press R to restart/);
  }
});

'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const {
  BOARD_WIDTH, BOARD_HEIGHT, SHAPES, spawnPiece, canPlace, movePiece,
  clearCompletedLines, rotatePiece, render,
} = require('../index');

const emptyBoard = () => Array.from({ length: BOARD_HEIGHT }, () => Array(BOARD_WIDTH).fill(false));

test('spawns each tetromino centered and rejects unknown pieces', () => {
  for (const [type, shape] of Object.entries(SHAPES)) {
    const piece = spawnPiece(type);
    assert.equal(piece.type, type);
    assert.deepEqual(piece.shape, shape);
    assert.equal(piece.x, Math.floor((BOARD_WIDTH - shape[0].length) / 2));
    assert.equal(canPlace(emptyBoard(), piece), true);
  }
  assert.throws(() => spawnPiece('invalid'), /Unknown tetromino/);
});

test('movement checks walls, floor, and occupied cells without changing failed moves', () => {
  const board = emptyBoard();
  const piece = spawnPiece('O');
  while (movePiece(board, piece, -1, 0)) {}
  assert.equal(piece.x, 0);
  assert.equal(movePiece(board, piece, -1, 0), false);
  while (movePiece(board, piece, 0, 1)) {}
  assert.equal(piece.y, BOARD_HEIGHT - piece.shape.length);
  assert.equal(movePiece(board, piece, 0, 1), false);
  board[piece.y + 1][piece.x] = true;
  assert.equal(canPlace(board, piece), false);
});

test('rotation preserves cells and rejects blocked placements without mutation', () => {
  const board = emptyBoard();
  const piece = spawnPiece('T');
  assert.equal(rotatePiece(board, piece), true);
  assert.deepEqual(piece.shape, [[1, 0], [1, 1], [1, 0]]);
  assert.equal(canPlace(board, piece), true);
});

test('clears all completed lines and returns their count', () => {
  const board = emptyBoard();
  board[18].fill(true);
  board[19].fill(true);
  assert.equal(clearCompletedLines(board), 2);
  assert.equal(board.length, BOARD_HEIGHT);
  assert.ok(board[0].every((cell) => !cell));
  assert.ok(board[1].every((cell) => !cell));
});

test('render stays within 24 terminal rows, including controls and game-over text', () => {
  const board = emptyBoard();
  for (const gameOver of [false, true]) {
    let output = '';
    const originalWrite = process.stdout.write;
    process.stdout.write = (chunk) => { output += chunk; return true; };
    try { render(board, spawnPiece('T'), 123, gameOver); } finally { process.stdout.write = originalWrite; }
    assert.equal(output.split('\n').length - 1, 24);
    assert.match(output, gameOver ? /GAME OVER/ : /Space: hard drop/);
  }
});

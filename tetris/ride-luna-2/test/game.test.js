'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const game = require('../index');

test('board has the expected dimensions and starts empty', () => {
  const board = game.emptyBoard();
  assert.equal(board.length, game.HEIGHT);
  assert.ok(board.every(row => row.length === game.WIDTH && row.every(cell => !cell)));
});

test('movement respects walls and settled blocks', () => {
  const board = game.emptyBoard();
  const piece = game.spawnPiece('O', board);
  piece.x = 0;
  assert.equal(game.movePiece(board, piece, -1, 0), false);
  assert.equal(game.movePiece(board, piece, 1, 0), true);
  board[piece.y][piece.x + 2] = 'T';
  assert.equal(game.movePiece(board, piece, 1, 0), false);
});

test('rotation and hard drop move a piece without crossing the floor', () => {
  const board = game.emptyBoard();
  const piece = game.spawnPiece('T', board);
  assert.equal(game.rotatePiece(board, piece), true);
  const distance = game.hardDrop(board, piece);
  assert.ok(distance > 0);
  assert.equal(game.collides(board, piece, 0, 1), true);
});

test('completed rows clear and the board compacts', () => {
  const board = game.emptyBoard();
  board[game.HEIGHT - 1].fill('I');
  board[game.HEIGHT - 2][0] = 'T';
  assert.equal(game.clearCompletedRows(board), 1);
  assert.equal(board[game.HEIGHT - 1][0], 'T');
  assert.deepEqual(board[game.HEIGHT - 2], Array(game.WIDTH).fill(0));
});

const test = require('node:test');
const assert = require('node:assert/strict');
const { WIDTH, HEIGHT, SHAPES, newBoard, canPlace, clearLines, rotateCells } = require('./rules');

test('seven tetrominoes each contain four distinct squares', () => {
  assert.equal(Object.keys(SHAPES).length, 7);
  for (const cells of Object.values(SHAPES)) {
    assert.equal(cells.length, 4);
    assert.equal(new Set(cells.map(cell => cell.join(','))).size, 4);
  }
});

test('placement checks walls, floor, ceiling and locked cells', () => {
  const board = newBoard();
  assert.equal(board.length, HEIGHT);
  assert.equal(board[0].length, WIDTH);
  assert.equal(canPlace(board, SHAPES.O, 3, 0), true);
  assert.equal(canPlace(board, SHAPES.O, -2, 0), false);
  assert.equal(canPlace(board, SHAPES.O, WIDTH - 2, 0), false);
  assert.equal(canPlace(board, SHAPES.O, 3, HEIGHT - 1), false);
  assert.equal(canPlace(board, SHAPES.I, 3, -1), true); // I's first occupied row is y=1.
  assert.equal(canPlace(board, SHAPES.T, 3, -1), false);
  board[1][4] = 'I';
  assert.equal(canPlace(board, SHAPES.O, 3, 0), false);
});

test('rotation returns to original after four turns and preserves squares', () => {
  for (const cells of Object.values(SHAPES)) {
    let rotated = cells;
    for (let i = 0; i < 4; i++) rotated = rotateCells(rotated);
    assert.deepEqual(rotated, cells);
  }
});

test('clearing consecutive lines shifts remaining cells down and awards score', () => {
  for (const [count, points] of [[0, 0], [1, 100], [2, 300], [3, 500], [4, 800]]) {
    const board = newBoard();
    board[HEIGHT - count - 1][0] = 'T';
    for (let y = HEIGHT - count; y < HEIGHT; y++) board[y].fill('I');
    assert.equal(clearLines(board), points);
    assert.equal(board.length, HEIGHT);
    assert.equal(board[HEIGHT - 1][0], 'T');
    assert.equal(board[HEIGHT - 1].filter(Boolean).length, 1);
    assert.equal(board[0].filter(Boolean).length, 0);
  }
});

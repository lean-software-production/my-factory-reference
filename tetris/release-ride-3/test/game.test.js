const test = require('node:test');
const assert = require('node:assert/strict');
const { createBoard, canPlace, move, rotate, hardDrop, step, clearLines } = require('../src/game');
const { SHAPES, spawnPiece } = require('../src/pieces');
const { WIDTH, HEIGHT } = require('../src/render');

test('board and spawn contain the expected dimensions and all seven pieces', () => {
  const board = createBoard();
  assert.equal(board.length, HEIGHT);
  assert.ok(board.every(row => row.length === WIDTH && row.every(cell => cell === 0)));
  assert.equal(Object.keys(SHAPES).length, 7);
  for (let i = 0; i < 7; i++) {
    const piece = spawnPiece(WIDTH, () => (i + 0.5) / 7);
    assert.equal(piece.cells.length, 4);
    assert.ok(canPlace(board, piece));
    assert.equal(piece.type, Object.keys(SHAPES)[i]);
  }
});

test('movement rejects walls, floor and locked cells without moving the piece', () => {
  const board = createBoard();
  const piece = { type: 'O', cells: SHAPES.O, x: 0, y: 0 };
  assert.equal(move(board, piece, -1), false);
  assert.equal(move(board, piece, 0, -1), false);
  assert.equal(move(board, piece, WIDTH), false);
  board[0][2] = 'T';
  assert.equal(move(board, piece, 1), false);
  assert.deepEqual([piece.x, piece.y], [0, 0]);
  assert.equal(move(board, piece, 0, 1), true);
  assert.deepEqual([piece.x, piece.y], [0, 1]);
  piece.y = HEIGHT - 2;
  assert.equal(move(board, piece, 0, 1), false);
});

test('rotation kicks off a wall but cannot overlap a locked cell', () => {
  const board = createBoard();
  const piece = { type: 'I', cells: SHAPES.I.map(cell => [...cell]), x: 0, y: 1 };
  assert.equal(rotate(board, piece), true); // Vertical.
  piece.x = WIDTH - 1;
  assert.equal(rotate(board, piece), true); // Horizontal, kicked away from right wall.
  assert.ok(canPlace(board, piece));
  assert.equal(piece.x, WIDTH - 4);

  const blocked = createBoard();
  const turn = { type: 'I', cells: SHAPES.I.map(cell => [...cell]), x: 3, y: 0 };
  for (let x = 0; x < WIDTH; x++) blocked[1][x] = 'O';
  assert.equal(rotate(blocked, turn), false);
  assert.deepEqual(turn.cells, SHAPES.I);
  assert.equal(turn.x, 3);
});

test('falling and hard drop lock pieces in the board', () => {
  const board = createBoard();
  const piece = { type: 'O', cells: SHAPES.O.map(cell => [...cell]), x: 3, y: 0 };
  assert.equal(step(board, piece), piece);
  assert.equal(piece.y, 1);
  assert.equal(hardDrop(board, piece), null);
  assert.equal(piece.y, HEIGHT - 2);
  for (const [dx, dy] of piece.cells) assert.equal(board[piece.y + dy][piece.x + dx], 'O');
  assert.equal(canPlace(board, { ...piece, y: HEIGHT - 2 }), false);
});

test('clearing adjacent full rows shifts remaining blocks down', () => {
  const board = createBoard();
  board[HEIGHT - 3][0] = 'T';
  board[HEIGHT - 2].fill('I');
  board[HEIGHT - 1].fill('I');
  assert.equal(clearLines(board), 2);
  assert.equal(board.length, HEIGHT);
  assert.equal(board[HEIGHT - 1][0], 'T');
  assert.ok(board[0].every(cell => cell === 0));
  assert.equal(clearLines(board), 0);
});

test('a piece cannot spawn when its entry cells are occupied', () => {
  const board = createBoard();
  const next = spawnPiece(WIDTH, () => 0);
  board[next.y][next.x] = 'O';
  assert.equal(canPlace(board, next), false);
});

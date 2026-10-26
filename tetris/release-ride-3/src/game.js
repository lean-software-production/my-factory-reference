const { WIDTH, HEIGHT } = require('./render');

function createBoard() {
  return Array.from({ length: HEIGHT }, () => Array(WIDTH).fill(0));
}

function canPlace(board, piece, x = piece.x, y = piece.y) {
  return piece.cells.every(([dx, dy]) => {
    const column = x + dx;
    const row = y + dy;
    return row >= 0 && row < board.length && column >= 0 && column < board[0].length && !board[row][column];
  });
}

function move(board, piece, dx, dy = 0) {
  if (!piece || !canPlace(board, piece, piece.x + dx, piece.y + dy)) return false;
  piece.x += dx;
  piece.y += dy;
  return true;
}

// Rotate clockwise inside the piece's bounding box. Try small horizontal wall
// kicks, but never rotate through a locked cell or beyond the board.
function rotate(board, piece) {
  if (!piece) return false;
  const height = Math.max(...piece.cells.map(([, y]) => y)) + 1;
  const cells = piece.cells.map(([x, y]) => [height - 1 - y, x]);
  const turned = { ...piece, cells };
  for (const dx of [0, -1, 1, -2, 2, -3, 3]) {
    if (canPlace(board, turned, piece.x + dx, piece.y)) {
      piece.cells = cells;
      piece.x += dx;
      return true;
    }
  }
  return false;
}

function hardDrop(board, piece) {
  if (!piece) return null;
  while (move(board, piece, 0, 1)) {}
  return step(board, piece);
}

function step(board, piece) {
  if (!piece) return null;
  if (move(board, piece, 0, 1)) return piece;
  for (const [dx, dy] of piece.cells) {
    board[piece.y + dy][piece.x + dx] = piece.type;
  }
  return null;
}

module.exports = { createBoard, canPlace, move, rotate, hardDrop, step };

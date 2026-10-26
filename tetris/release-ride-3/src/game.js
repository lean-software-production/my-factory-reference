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

function step(board, piece) {
  if (!piece) return null;
  if (canPlace(board, piece, piece.x, piece.y + 1)) {
    piece.y += 1;
    return piece;
  }
  for (const [dx, dy] of piece.cells) {
    board[piece.y + dy][piece.x + dx] = piece.type;
  }
  return null;
}

module.exports = { createBoard, canPlace, step };

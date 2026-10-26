const { BOARD_WIDTH, BOARD_HEIGHT } = require('./render');
const { randomPiece } = require('./pieces');

function createGame() {
  return {
    board: Array.from({ length: BOARD_HEIGHT }, () => Array(BOARD_WIDTH).fill(null)),
    piece: randomPiece(BOARD_WIDTH),
    score: 0,
    gameOver: false,
  };
}

function canPlace(board, piece, x = piece.x, y = piece.y, cells = piece.cells) {
  return cells.every(([dx, dy]) => {
    const col = x + dx;
    const row = y + dy;
    return col >= 0 && col < BOARD_WIDTH && row >= 0 && row < BOARD_HEIGHT && !board[row][col];
  });
}

function move(game, dx) {
  if (game.gameOver || !canPlace(game.board, game.piece, game.piece.x + dx)) return false;
  game.piece.x += dx;
  return true;
}

function rotate(game) {
  if (game.gameOver) return false;
  const piece = game.piece;
  if (piece.type === 'O') return true;

  // Rotate within the piece's spawn box, then try small horizontal wall kicks.
  const size = piece.type === 'I' ? 4 : 3;
  const cells = piece.cells.map(([x, y]) => [size - 1 - y, x]);
  for (const offset of [0, -1, 1, -2, 2]) {
    if (canPlace(game.board, piece, piece.x + offset, piece.y, cells)) {
      piece.x += offset;
      piece.cells = cells;
      return true;
    }
  }
  return false;
}

function tick(game) {
  if (game.gameOver) return;
  if (canPlace(game.board, game.piece, game.piece.x, game.piece.y + 1)) {
    game.piece.y++;
    return;
  }

  for (const [dx, dy] of game.piece.cells) {
    game.board[game.piece.y + dy][game.piece.x + dx] = game.piece.type;
  }
  game.piece = randomPiece(BOARD_WIDTH);
  if (!canPlace(game.board, game.piece)) game.gameOver = true;
}

module.exports = { createGame, canPlace, move, rotate, tick };

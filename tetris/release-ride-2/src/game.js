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

module.exports = { createGame, canPlace, tick };

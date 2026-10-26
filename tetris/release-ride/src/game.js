const { WIDTH, HEIGHT } = require('./render');
const { spawnPiece } = require('./pieces');

function createGame(random = Math.random) {
  return {
    board: Array.from({ length: HEIGHT }, () => Array(WIDTH).fill(0)),
    piece: spawnPiece(random),
    score: 0,
    gameOver: false,
  };
}

function canPlace(board, piece) {
  return piece.cells.every(([dx, dy]) => {
    const x = piece.x + dx;
    const y = piece.y + dy;
    return x >= 0 && x < WIDTH && y >= 0 && y < HEIGHT && !board[y][x];
  });
}

function tick(game, random = Math.random) {
  if (game.gameOver) return;
  const next = { ...game.piece, y: game.piece.y + 1 };
  if (canPlace(game.board, next)) {
    game.piece = next;
    return;
  }

  for (const [dx, dy] of game.piece.cells) {
    game.board[game.piece.y + dy][game.piece.x + dx] = game.piece.type;
  }
  game.piece = spawnPiece(random);
  // Wait for the game-over UI before displaying a piece that cannot spawn.
  if (!canPlace(game.board, game.piece)) {
    game.gameOver = true;
    game.piece = null;
  }
}

module.exports = { createGame, canPlace, tick };

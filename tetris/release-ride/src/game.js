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

function move(game, dx, dy) {
  if (game.gameOver || !game.piece) return false;
  const next = { ...game.piece, x: game.piece.x + dx, y: game.piece.y + dy };
  if (!canPlace(game.board, next)) return false;
  game.piece = next;
  return true;
}

function rotate(game) {
  if (game.gameOver || !game.piece) return false;
  // Rotate clockwise within the piece's spawn area (3×3, or 4×4 for I).
  if (game.piece.type === 'O') return true;
  const size = game.piece.type === 'I' ? 4 : 3;
  const next = {
    ...game.piece,
    cells: game.piece.cells.map(([x, y]) => [size - 1 - y, x]),
  };
  if (!canPlace(game.board, next)) return false;
  game.piece = next;
  return true;
}

function clearLines(game) {
  const remaining = game.board.filter(row => row.some(cell => !cell));
  const cleared = HEIGHT - remaining.length;
  game.board = [
    ...Array.from({ length: cleared }, () => Array(WIDTH).fill(0)),
    ...remaining,
  ];
  game.score += cleared * 100;
  return cleared;
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
  clearLines(game);
  game.piece = spawnPiece(random);
  // Wait for the game-over UI before displaying a piece that cannot spawn.
  if (!canPlace(game.board, game.piece)) {
    game.gameOver = true;
    game.piece = null;
  }
}

function hardDrop(game, random = Math.random) {
  if (game.gameOver || !game.piece) return false;
  while (move(game, 0, 1)) {}
  tick(game, random);
  return true;
}

module.exports = { createGame, canPlace, move, rotate, clearLines, tick, hardDrop };

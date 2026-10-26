// Coordinates are relative to the top-left corner of each piece's spawn box.
const SHAPES = {
  I: [[0, 1], [1, 1], [2, 1], [3, 1]],
  O: [[0, 0], [1, 0], [0, 1], [1, 1]],
  T: [[1, 0], [0, 1], [1, 1], [2, 1]],
  S: [[1, 0], [2, 0], [0, 1], [1, 1]],
  Z: [[0, 0], [1, 0], [1, 1], [2, 1]],
  J: [[0, 0], [0, 1], [1, 1], [2, 1]],
  L: [[2, 0], [0, 1], [1, 1], [2, 1]],
};

function spawnPiece(type, boardWidth) {
  const cells = SHAPES[type];
  if (!cells) throw new Error(`Unknown tetromino: ${type}`);
  const width = Math.max(...cells.map(([x]) => x)) + 1;
  return { type, x: Math.floor((boardWidth - width) / 2), y: 0, cells: cells.map(cell => [...cell]) };
}

function randomPiece(boardWidth) {
  const types = Object.keys(SHAPES);
  return spawnPiece(types[Math.floor(Math.random() * types.length)], boardWidth);
}

module.exports = { SHAPES, spawnPiece, randomPiece };

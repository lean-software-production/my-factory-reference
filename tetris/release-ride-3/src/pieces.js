// Coordinates are relative to the top-left corner of each piece's bounding box.
const SHAPES = Object.freeze({
  I: [[0, 0], [1, 0], [2, 0], [3, 0]],
  O: [[0, 0], [1, 0], [0, 1], [1, 1]],
  T: [[0, 0], [1, 0], [2, 0], [1, 1]],
  S: [[1, 0], [2, 0], [0, 1], [1, 1]],
  Z: [[0, 0], [1, 0], [1, 1], [2, 1]],
  J: [[0, 0], [0, 1], [1, 1], [2, 1]],
  L: [[2, 0], [0, 1], [1, 1], [2, 1]],
});

function spawnPiece(width = 10, random = Math.random) {
  const types = Object.keys(SHAPES);
  const type = types[Math.floor(random() * types.length)];
  const cells = SHAPES[type].map(([x, y]) => [x, y]);
  const pieceWidth = Math.max(...cells.map(([x]) => x)) + 1;
  return { type, cells, x: Math.floor((width - pieceWidth) / 2), y: 0 };
}

module.exports = { SHAPES, spawnPiece };

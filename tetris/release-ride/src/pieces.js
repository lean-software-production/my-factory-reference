// Coordinates are relative to the top-left corner of each piece's 4×4 spawn area.
const SHAPES = {
  I: [[0, 1], [1, 1], [2, 1], [3, 1]],
  O: [[1, 0], [2, 0], [1, 1], [2, 1]],
  T: [[1, 0], [0, 1], [1, 1], [2, 1]],
  S: [[1, 0], [2, 0], [0, 1], [1, 1]],
  Z: [[0, 0], [1, 0], [1, 1], [2, 1]],
  J: [[0, 0], [0, 1], [1, 1], [2, 1]],
  L: [[2, 0], [0, 1], [1, 1], [2, 1]],
};

function spawnPiece(random = Math.random) {
  const types = Object.keys(SHAPES);
  const type = types[Math.floor(random() * types.length)];
  return { type, x: 3, y: 0, cells: SHAPES[type].map(([x, y]) => [x, y]) };
}

module.exports = { SHAPES, spawnPiece };

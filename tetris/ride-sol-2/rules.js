const WIDTH = 10;
const HEIGHT = 20;

// Coordinates are relative to the top-left corner of each piece's 4×4 box.
const SHAPES = {
  I: [[0, 1], [1, 1], [2, 1], [3, 1]],
  O: [[1, 0], [2, 0], [1, 1], [2, 1]],
  T: [[1, 0], [0, 1], [1, 1], [2, 1]],
  S: [[1, 0], [2, 0], [0, 1], [1, 1]],
  Z: [[0, 0], [1, 0], [1, 1], [2, 1]],
  J: [[0, 0], [0, 1], [1, 1], [2, 1]],
  L: [[2, 0], [0, 1], [1, 1], [2, 1]],
};

function newBoard() {
  return Array.from({ length: HEIGHT }, () => Array(WIDTH).fill(null));
}

function canPlace(board, cells, x, y) {
  return cells.every(([dx, dy]) => {
    const col = x + dx;
    const row = y + dy;
    return col >= 0 && col < WIDTH && row >= 0 && row < HEIGHT && !board[row][col];
  });
}

function clearLines(board) {
  let cleared = 0;
  for (let y = HEIGHT - 1; y >= 0; y--) {
    if (board[y].every(Boolean)) {
      board.splice(y, 1);
      board.unshift(Array(WIDTH).fill(null));
      cleared++;
      y++; // Check this row again: the rows above have shifted down.
    }
  }
  return [0, 100, 300, 500, 800][cleared] || 0;
}

function rotateCells(cells) {
  return cells.map(([x, y]) => [3 - y, x]);
}

module.exports = { WIDTH, HEIGHT, SHAPES, newBoard, canPlace, clearLines, rotateCells };

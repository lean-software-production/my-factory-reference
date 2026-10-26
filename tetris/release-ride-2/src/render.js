const BOARD_WIDTH = 10;
const BOARD_HEIGHT = 20;

// The border and twenty board rows occupy 22 terminal rows in total.
function render(board, score, piece = null, gameOver = false, nextPiece = null) {
  const sidebar = {
    1: `Score: ${score}`,
    3: 'Controls:',
    4: 'Left/Right or A/D  Move',
    5: 'Down or S  Soft drop',
    6: 'Up or W  Rotate',
    7: 'Space  Hard drop',
    8: 'Q  Quit',
    9: gameOver ? 'Game over' : '',
    11: 'Next:',
  };
  if (nextPiece) {
    const cells = new Set(nextPiece.cells.map(([x, y]) => `${x},${y}`));
    for (let y = 0; y < 4; y++) {
      sidebar[12 + y] = Array.from({ length: 4 }, (_, x) => cells.has(`${x},${y}`) ? '[]' : '  ').join('');
    }
  }
  const border = `+${'-'.repeat(BOARD_WIDTH * 2)}+`;
  const lines = [border];
  const falling = new Set(piece ? piece.cells.map(([dx, dy]) => `${piece.x + dx},${piece.y + dy}`) : []);

  for (let y = 0; y < BOARD_HEIGHT; y++) {
    const cells = board[y].map((cell, x) => cell || falling.has(`${x},${y}`) ? '[]' : '  ').join('');
    lines.push(`|${cells}|  ${sidebar[y] || ''}`);
  }
  lines.push(border);
  return lines.join('\n');
}

module.exports = { BOARD_WIDTH, BOARD_HEIGHT, render };

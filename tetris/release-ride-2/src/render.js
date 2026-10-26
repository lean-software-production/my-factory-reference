const BOARD_WIDTH = 10;
const BOARD_HEIGHT = 20;

// The border and twenty board rows occupy 22 terminal rows in total.
function render(board, score, piece = null, gameOver = false) {
  const sidebar = {
    1: `Score: ${score}`,
    3: 'Controls:',
    4: 'Q  Quit',
    6: gameOver ? 'Game over' : '',
  };
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

const BOARD_WIDTH = 10;
const BOARD_HEIGHT = 20;

// The border and twenty board rows occupy 22 terminal rows in total.
function render(board, score) {
  const sidebar = {
    1: `Score: ${score}`,
    3: 'Controls:',
    4: 'Q  Quit',
  };
  const border = `+${'-'.repeat(BOARD_WIDTH * 2)}+`;
  const lines = [border];

  for (let y = 0; y < BOARD_HEIGHT; y++) {
    const cells = board[y].map(cell => cell ? '[]' : '  ').join('');
    lines.push(`|${cells}|  ${sidebar[y] || ''}`);
  }
  lines.push(border);
  return lines.join('\n');
}

module.exports = { BOARD_WIDTH, BOARD_HEIGHT, render };

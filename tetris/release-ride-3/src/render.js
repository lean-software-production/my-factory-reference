const WIDTH = 10;
const HEIGHT = 20;

// Keep the entire screen at 24 rows: header, border, 20 cells, border, footer.
function render(board = Array.from({ length: HEIGHT }, () => Array(WIDTH).fill(0)),
                { score = 0, lines = 0, gameOver = false } = {}) {
  const border = `+${'-'.repeat(WIDTH * 2)}+`;
  const rows = [
    `TETRIS  Score: ${score}  Lines: ${lines}`,
    border,
    ...board.map(row => `|${row.map(cell => cell ? '[]' : '  ').join('')}|`),
    border,
    gameOver ? 'GAME OVER  R Restart  Q Quit' : '←→ Move  ↓ Soft  ↑ Turn  Space Hard  R Restart  Q Quit',
  ];
  return rows.join('\r\n');
}

module.exports = { render, WIDTH, HEIGHT };

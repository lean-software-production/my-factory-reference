const WIDTH = 10;
const HEIGHT = 20;

function render(board, score = 0, piece = null) {
  const falling = new Set(piece ? piece.cells.map(([x, y]) => `${piece.x + x},${piece.y + y}`) : []);
  const sidebar = [
    'Score: ' + score,
    '',
    'Controls',
    'Left/Right  move',
    'Up          rotate',
    'Down        soft drop',
    'Space       hard drop',
    'R           restart',
    'Q           quit',
  ];
  const border = '+' + '-'.repeat(WIDTH) + '+';
  const lines = ['TETRIS', border + '  ' + sidebar[0]];

  for (let y = 0; y < HEIGHT; y++) {
    const row = board[y];
    const cells = Array.from({ length: WIDTH }, (_, x) => row?.[x] || falling.has(`${x},${y}`) ? '#' : ' ').join('');
    lines.push('|' + cells + '|  ' + (sidebar[y + 1] || ''));
  }
  lines.push(border);
  return lines.join('\n');
}

module.exports = { render, WIDTH, HEIGHT };

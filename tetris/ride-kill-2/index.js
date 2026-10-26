// Terminal rendering, piece spawning, and gravity.
const { stdin, stdout } = process;
const WIDTH = 10;
const HEIGHT = 20;
const board = Array.from({ length: HEIGHT }, () => Array(WIDTH).fill(null));
// Rows are kept as square matrices so later rotations can turn every piece.
const SHAPES = {
  I: ['....', '####', '....', '....'],
  O: ['##', '##'],
  T: ['.#.', '###', '...'],
  S: ['.##', '##.', '...'],
  Z: ['##.', '.##', '...'],
  J: ['#..', '###', '...'],
  L: ['..#', '###', '...']
};
const PIECES = Object.entries(SHAPES).map(([name, rows]) => ({
  name,
  cells: rows.map(row => [...row].map(cell => cell === '#'))
}));
function spawnPiece() {
  const shape = PIECES[Math.floor(Math.random() * PIECES.length)];
  return { shape, x: Math.floor((WIDTH - shape.cells.length) / 2), y: 0 };
}
let active = spawnPiece();
let score = 0;

function canPlace(piece, x, y) {
  return piece.shape.cells.every((row, dy) => row.every((filled, dx) => {
    if (!filled) return true;
    const boardX = x + dx;
    const boardY = y + dy;
    return boardX >= 0 && boardX < WIDTH && boardY >= 0 && boardY < HEIGHT &&
      !board[boardY][boardX];
  }));
}

function lockPiece() {
  active.shape.cells.forEach((row, dy) => row.forEach((filled, dx) => {
    if (filled) board[active.y + dy][active.x + dx] = active.shape.name;
  }));
  // Spawning the next piece is added in a later step.
  active = null;
}

function tick() {
  if (!active) return;
  if (canPlace(active, active.x, active.y + 1)) active.y++;
  else lockPiece();
  render();
}

function render() {
  const border = '+' + '-'.repeat(WIDTH * 2) + '+';
  const rows = board.map((row, y) => '|' + row.map((cell, x) => {
    const pieceX = active ? x - active.x : -1;
    const pieceY = active ? y - active.y : -1;
    const occupied = active && pieceY >= 0 && pieceY < active.shape.cells.length &&
      pieceX >= 0 && pieceX < active.shape.cells[pieceY].length &&
      active.shape.cells[pieceY][pieceX];
    return cell || occupied ? '[]' : '  ';
  }).join('') + '|');
  // 22 board lines plus two status lines: exactly 24 terminal rows.
  stdout.write('\x1b[H' + [
    border, ...rows, border,
    `TETRIS  Score: ${score}`,
    'Arrows: move/rotate/drop  Space: hard drop  q: quit'
  ].join('\n'));
}

if (!stdin.isTTY || !stdout.isTTY) {
  stdout.write('TETRIS\nRun npm start in a terminal to play.\n');
} else {
  let closing = false;
  let gravity;
  const close = () => {
    if (closing) return;
    closing = true;
    clearInterval(gravity);
    stdin.setRawMode(false);
    stdin.pause();
    stdout.write('\n');
    process.exit(0);
  };

  stdin.setRawMode(true);
  stdin.resume();
  stdin.on('data', (input) => {
    if (input.includes(0x71) || input.includes(0x03)) close(); // q or Ctrl-C
  });
  process.on('SIGINT', close);
  process.on('SIGTERM', close);

  stdout.write('\x1b[2J');
  render();
  gravity = setInterval(tick, 600);
}

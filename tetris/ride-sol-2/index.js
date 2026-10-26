// Terminal lifecycle and board display.
const WIDTH = 10;
const HEIGHT = 20;
const board = Array.from({ length: HEIGHT }, () => Array(WIDTH).fill(null));
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
const kinds = Object.keys(SHAPES);
const kind = kinds[Math.floor(Math.random() * kinds.length)];
let active = { kind, x: Math.floor((WIDTH - 4) / 2), y: 0, cells: SHAPES[kind] };
let score = 0;
let gravity;

function canPlace(piece, x, y, cells = piece.cells) {
  return cells.every(([dx, dy]) => {
    const col = x + dx;
    const row = y + dy;
    return col >= 0 && col < WIDTH && row >= 0 && row < HEIGHT && !board[row][col];
  });
}

function lockPiece() {
  for (const [dx, dy] of active.cells) {
    board[active.y + dy][active.x + dx] = active.kind;
  }
  active = null; // Spawning the next piece is added in the next gameplay step.
  clearInterval(gravity);
}

function tick() {
  if (!active) return;
  if (canPlace(active, active.x, active.y + 1)) active.y++;
  else lockPiece();
  render();
}

function render() {
  // Twenty play rows plus a top and bottom border: 22 terminal rows total.
  const panel = [
    'TETRIS',
    '',
    `Score: ${score}`,
    '',
    'Controls',
    '← →  Move',
    '↑    Rotate',
    '↓    Soft drop',
    'Space Hard drop',
    'Q    Quit',
  ];
  const fallingCells = new Set(active ? active.cells.map(([dx, dy]) => `${active.x + dx},${active.y + dy}`) : []);
  const lines = [`┌${'─'.repeat(WIDTH * 2)}┐`];
  for (let y = 0; y < HEIGHT; y++) {
    lines.push(`│${board[y].map((cell, x) => cell || fallingCells.has(`${x},${y}`) ? '██' : '  ').join('')}│`);
  }
  lines.push(`└${'─'.repeat(WIDTH * 2)}┘`);
  for (let y = 0; y < lines.length; y++) {
    if (panel[y]) lines[y] += `  ${panel[y]}`;
  }
  stdout.write('\x1b[H' + lines.join('\r\n'));
}
const stdin = process.stdin;
const stdout = process.stdout;

if (!stdin.isTTY || !stdout.isTTY) {
  console.log('Tetris needs an interactive terminal.');
  process.exit(0);
}

let closed = false;
function close() {
  if (closed) return;
  closed = true;
  clearInterval(gravity);
  stdin.setRawMode(false);
  stdin.pause();
  stdout.write('\x1b[?25h\x1b[?1049l'); // Restore cursor and previous screen.
  process.exit(0);
}

process.on('SIGINT', close);
process.on('SIGTERM', close);
process.on('exit', () => {
  if (!closed) {
    stdin.setRawMode(false);
    stdout.write('\x1b[?25h\x1b[?1049l');
  }
});

stdin.setRawMode(true);
stdin.resume();
stdin.setEncoding('utf8');
stdin.on('data', (key) => {
  if (key === 'q' || key === '\x03') close();
});

stdout.write('\x1b[?1049h\x1b[?25l\x1b[2J\x1b[H');
render();
gravity = setInterval(tick, 700);

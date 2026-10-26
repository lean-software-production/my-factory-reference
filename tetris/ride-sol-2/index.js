// Terminal lifecycle and board display.
const { WIDTH, HEIGHT, SHAPES, newBoard, canPlace: fits, clearLines: removeLines, rotateCells } = require('./rules');
const board = newBoard();
const kinds = Object.keys(SHAPES);
let active = null;
let gameOver = false;
let score = 0;
let gravity;

function canPlace(piece, x, y, cells = piece.cells) {
  return fits(board, cells, x, y);
}

function spawnPiece() {
  const kind = kinds[Math.floor(Math.random() * kinds.length)];
  const next = { kind, x: Math.floor((WIDTH - 4) / 2), y: 0, cells: SHAPES[kind] };
  if (canPlace(next, next.x, next.y)) active = next;
  else {
    active = null;
    gameOver = true;
    clearInterval(gravity);
  }
}

function restart() {
  clearInterval(gravity);
  for (const row of board) row.fill(null);
  score = 0;
  gameOver = false;
  active = null;
  spawnPiece();
  gravity = setInterval(tick, 700);
  render();
}

function lockPiece() {
  for (const [dx, dy] of active.cells) {
    board[active.y + dy][active.x + dx] = active.kind;
  }
  score += removeLines(board);
  spawnPiece();
}

function move(dx) {
  if (active && canPlace(active, active.x + dx, active.y)) {
    active.x += dx;
    render();
  }
}

function rotate() {
  if (!active || active.kind === 'O') return;
  const rotated = rotateCells(active.cells);
  // Try the current position first, then small horizontal wall kicks.
  for (const dx of [0, -1, 1, -2, 2]) {
    if (canPlace(active, active.x + dx, active.y, rotated)) {
      active.x += dx;
      active.cells = rotated;
      render();
      return;
    }
  }
}

function hardDrop() {
  if (!active) return;
  while (canPlace(active, active.x, active.y + 1)) active.y++;
  lockPiece();
  render();
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
    'R    Restart',
    'Q    Quit',
    gameOver ? 'GAME OVER - R to restart' : '',
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
  // Clear each row before rewriting it: shorter scores and the disappearing
  // game-over message must not leave characters from the previous frame.
  // Leave one column unused so writing at the right edge cannot auto-wrap.
  const maxColumns = Math.max(0, (stdout.columns || 80) - 1);
  stdout.write('\x1b[H' + lines.map(line => '\x1b[2K' + Array.from(line).slice(0, maxColumns).join('')).join('\r\n'));
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

const readline = require('node:readline');
readline.emitKeypressEvents(stdin);
stdin.setRawMode(true);
stdin.resume();
stdout.on('resize', render);
stdin.on('keypress', (_, key) => {
  if (key?.name === 'q' || (key?.ctrl && key.name === 'c')) close();
  else if (key?.name === 'r') restart();
  else if (key?.name === 'left') move(-1);
  else if (key?.name === 'right') move(1);
  else if (key?.name === 'up') rotate();
  else if (key?.name === 'down') tick();
  else if (key?.name === 'space') hardDrop();
});

stdout.write('\x1b[?1049h\x1b[?25l\x1b[2J\x1b[H');
restart();

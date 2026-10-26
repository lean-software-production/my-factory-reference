// Terminal lifecycle and board display. Gameplay will be added here.
const WIDTH = 10;
const HEIGHT = 20;
const board = Array.from({ length: HEIGHT }, () => Array(WIDTH).fill(null));
let score = 0;

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
  const lines = [`┌${'─'.repeat(WIDTH * 2)}┐`];
  for (let y = 0; y < HEIGHT; y++) {
    lines.push(`│${board[y].map(cell => cell ? '██' : '  ').join('')}│`);
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

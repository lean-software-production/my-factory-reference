// The initial screen and terminal lifecycle; gameplay is added in later steps.
const { stdin, stdout } = process;
const WIDTH = 10;
const HEIGHT = 20;
const board = Array.from({ length: HEIGHT }, () => Array(WIDTH).fill(null));
let score = 0;

function render() {
  const border = '+' + '-'.repeat(WIDTH * 2) + '+';
  const rows = board.map(row => '|' + row.map(cell => cell ? '[]' : '  ').join('') + '|');
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
  const close = () => {
    if (closing) return;
    closing = true;
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
}

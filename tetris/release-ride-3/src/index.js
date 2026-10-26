const readline = require('node:readline');
const { render, WIDTH } = require('./render');
const { spawnPiece } = require('./pieces');
const { createBoard, step } = require('./game');

const input = process.stdin;
const output = process.stdout;

if (!input.isTTY || !output.isTTY) {
  console.error('Tetris needs an interactive terminal.');
  process.exitCode = 1;
} else {
  let closed = false;
  let timer;

  function close() {
    if (closed) return;
    closed = true;
    clearInterval(timer);
    input.setRawMode(false);
    input.pause();
    output.write('\x1b[?25h\x1b[?1049l'); // Show cursor and leave alternate screen.
  }

  function quit() {
    close();
    process.exit();
  }

  process.once('SIGINT', quit);
  process.once('SIGTERM', quit);
  process.once('exit', close);

  readline.emitKeypressEvents(input);
  input.setRawMode(true);
  input.resume();
  input.on('keypress', (_character, key) => {
    if (key?.name === 'q' || (key?.ctrl && key.name === 'c')) quit();
  });

  output.write('\x1b[?1049h\x1b[?25l\x1b[H\x1b[2J'); // Alternate screen, hide cursor.
  const board = createBoard();
  let active = spawnPiece(WIDTH);
  output.write(render(board, { active }));
  timer = setInterval(() => {
    active = step(board, active);
    output.write('\x1b[H' + render(board, { active }));
    if (!active) clearInterval(timer);
  }, 700);
}

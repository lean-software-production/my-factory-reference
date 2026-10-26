const readline = require('node:readline');
const { render, WIDTH } = require('./render');
const { spawnPiece } = require('./pieces');
const { createBoard, move, rotate, hardDrop, step } = require('./game');

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
  output.write('\x1b[?1049h\x1b[?25l\x1b[H\x1b[2J'); // Alternate screen, hide cursor.
  const board = createBoard();
  let active = spawnPiece(WIDTH);
  function draw() {
    output.write('\x1b[H' + render(board, { active }));
  }
  function fall() {
    active = step(board, active);
    draw();
    if (!active) clearInterval(timer);
  }
  input.on('keypress', (_character, key) => {
    if (key?.name === 'q' || (key?.ctrl && key.name === 'c')) return quit();
    if (!active) return;
    if (key?.name === 'left') move(board, active, -1);
    else if (key?.name === 'right') move(board, active, 1);
    else if (key?.name === 'down') return fall();
    else if (key?.name === 'up') rotate(board, active);
    else if (key?.name === 'space') {
      active = hardDrop(board, active);
      clearInterval(timer);
    } else return;
    draw();
  });

  draw();
  timer = setInterval(fall, 700);
}

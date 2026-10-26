const readline = require('node:readline');
const { BOARD_WIDTH, BOARD_HEIGHT, render } = require('./render');

function start() {
  if (!process.stdin.isTTY || !process.stdout.isTTY) {
    console.error('Tetris needs an interactive terminal.');
    process.exitCode = 1;
    return;
  }

  const input = process.stdin;
  const output = process.stdout;
  let closed = false;

  function close() {
    if (closed) return;
    closed = true;
    input.removeListener('keypress', onKey);
    process.removeListener('SIGINT', close);
    input.setRawMode(false);
    input.pause();
    output.write('\x1b[?25h\x1b[0m\n');
  }

  function onKey(_text, key) {
    if (key && (key.name === 'q' || (key.ctrl && key.name === 'c'))) close();
  }

  readline.emitKeypressEvents(input);
  input.setRawMode(true);
  input.resume();
  input.on('keypress', onKey);
  process.on('SIGINT', close);
  const board = Array.from({ length: BOARD_HEIGHT }, () => Array(BOARD_WIDTH).fill(null));
  output.write(`\x1b[?25l\x1b[2J\x1b[H${render(board, 0)}`);
}

start();

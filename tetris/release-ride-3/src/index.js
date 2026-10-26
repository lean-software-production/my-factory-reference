const readline = require('node:readline');
const { render } = require('./render');

const input = process.stdin;
const output = process.stdout;

if (!input.isTTY || !output.isTTY) {
  console.error('Tetris needs an interactive terminal.');
  process.exitCode = 1;
} else {
  let closed = false;

  function close() {
    if (closed) return;
    closed = true;
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
  output.write(render());
}

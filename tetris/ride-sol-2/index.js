// Terminal lifecycle for the game. The board and gameplay will be added here.
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
stdout.write('Tetris\r\nPress q or Ctrl-C to quit.');

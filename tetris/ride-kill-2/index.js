// The initial screen and terminal lifecycle; gameplay is added in later steps.
const { stdin, stdout } = process;

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

  stdout.write('\x1b[2J\x1b[H' +
    '┌────────────────────┐\n' +
    '│       TETRIS       │\n' +
    '│                    │\n' +
    '│   Game loading…    │\n' +
    '└────────────────────┘\n' +
    'Press q to quit.');
}

const { createTerminal } = require('./terminal');

const terminal = createTerminal({
  onKey(_character, key) {
    if (key?.name === 'q' || (key?.ctrl && key?.name === 'c')) {
      terminal.stop();
    }
  },
});

try {
  terminal.start();
  terminal.draw([
    '+--------------------+',
    '|                    |',
    '|   Tetris           |',
    '|   Coming soon      |',
    '|                    |',
    '+--------------------+',
    'Press Q to quit',
  ]);
} catch (error) {
  terminal.stop();
  console.error(error.message);
  process.exitCode = 1;
}

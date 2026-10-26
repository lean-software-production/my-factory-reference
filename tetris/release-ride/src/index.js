const readline = require('node:readline');

process.stdout.write('TETRIS\n\nGame starting soon.\n');

if (process.stdin.isTTY) {
  const input = readline.createInterface({ input: process.stdin, output: process.stdout });
  input.question('\nPress Enter to exit.\n', () => input.close());
}

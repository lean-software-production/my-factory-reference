const readline = require('node:readline');
const { render, WIDTH, HEIGHT } = require('./render');

const board = Array.from({ length: HEIGHT }, () => Array(WIDTH).fill(0));
if (process.stdout.isTTY) process.stdout.write('\x1b[2J\x1b[H');
process.stdout.write(render(board, 0) + '\n');

if (process.stdin.isTTY) {
  const input = readline.createInterface({ input: process.stdin, output: process.stdout });
  input.question('', () => input.close());
}

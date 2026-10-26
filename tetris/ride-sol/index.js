#!/usr/bin/env node

'use strict';

const WIDTH = 10;
const HEIGHT = 20;

function render() {
  const board = Array.from({ length: HEIGHT }, () => Array(WIDTH).fill(' '));
  const sidebar = [
    'Score: 0',
    'Lines: 0',
    '',
    'Controls:',
    'Left/Right: Move',
    'Down: Soft drop',
    'Up: Rotate',
    'Space: Hard drop',
    'Q: Quit',
  ];
  const lines = [
    'TETRIS',
    '+' + '-'.repeat(WIDTH * 2) + '+',
    ...board.map((row, index) =>
      '|' + row.map((cell) => cell + ' ').join('') + '|  ' + (sidebar[index] || '')
    ),
    '+' + '-'.repeat(WIDTH * 2) + '+',
    'Press Q or Ctrl+C to quit.',
  ];
  // 1 title + 2 borders + 20 board rows + 1 footer = 24 terminal rows.
  process.stdout.write((process.stdout.isTTY ? '\x1b[H\x1b[2J' : '') + lines.join('\n'));
}

render();

if (process.stdin.isTTY) {
  process.stdin.setRawMode(true);
  process.stdin.resume();
  process.stdin.on('data', (key) => {
    if (key.toString().toLowerCase() === 'q' || key.toString() === '\u0003') {
      process.exit();
    }
  });
  process.on('exit', () => process.stdin.setRawMode(false));
}

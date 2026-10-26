#!/usr/bin/env node

'use strict';

const WIDTH = 10;
const HEIGHT = 20;

// Coordinates are relative to the top-left of each piece's 4×4 spawn area.
const SHAPES = {
  I: [[0, 1], [1, 1], [2, 1], [3, 1]],
  O: [[1, 0], [2, 0], [1, 1], [2, 1]],
  T: [[1, 0], [0, 1], [1, 1], [2, 1]],
  S: [[1, 0], [2, 0], [0, 1], [1, 1]],
  Z: [[0, 0], [1, 0], [1, 1], [2, 1]],
  J: [[0, 0], [0, 1], [1, 1], [2, 1]],
  L: [[2, 0], [0, 1], [1, 1], [2, 1]],
};

function spawnPiece() {
  const types = Object.keys(SHAPES);
  const type = types[Math.floor(Math.random() * types.length)];
  return { type, x: Math.floor((WIDTH - 4) / 2), y: 0, cells: SHAPES[type] };
}

const activePiece = spawnPiece();

function render() {
  const board = Array.from({ length: HEIGHT }, () => Array(WIDTH).fill(' '));
  for (const [dx, dy] of activePiece.cells) {
    board[activePiece.y + dy][activePiece.x + dx] = activePiece.type;
  }
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

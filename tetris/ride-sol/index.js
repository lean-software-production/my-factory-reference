#!/usr/bin/env node

'use strict';

const readline = require('node:readline');

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

const settled = Array.from({ length: HEIGHT }, () => Array(WIDTH).fill(' '));
let activePiece = spawnPiece();
let linesCleared = 0;
let score = 0;

function clearLines() {
  let cleared = 0;
  for (let row = HEIGHT - 1; row >= 0; row--) {
    if (settled[row].every((cell) => cell !== ' ')) {
      settled.splice(row, 1);
      settled.unshift(Array(WIDTH).fill(' '));
      cleared++;
      row++; // Recheck this row after the rows above fall into it.
    }
  }
  linesCleared += cleared;
  score += [0, 100, 300, 500, 800][cleared];
}

function canPlace(piece, x, y) {
  return piece.cells.every(([dx, dy]) => {
    const col = x + dx;
    const row = y + dy;
    return col >= 0 && col < WIDTH && row >= 0 && row < HEIGHT && settled[row][col] === ' ';
  });
}

function rotatePiece(piece) {
  if (piece.type === 'O') return false;
  const size = piece.type === 'I' ? 4 : 3;
  const cells = piece.cells.map(([x, y]) => [size - 1 - y, x]);
  const rotated = { ...piece, cells };
  // Try the current position first, then nudge away from a wall or the floor.
  for (const [dx, dy] of [[0, 0], [-1, 0], [1, 0], [-2, 0], [2, 0], [0, -1], [-1, -1], [1, -1]]) {
    if (canPlace(rotated, piece.x + dx, piece.y + dy)) {
      piece.x += dx;
      piece.y += dy;
      piece.cells = cells;
      return true;
    }
  }
  return false;
}

function tick() {
  if (!activePiece) return;
  if (canPlace(activePiece, activePiece.x, activePiece.y + 1)) {
    activePiece.y++;
  } else {
    for (const [dx, dy] of activePiece.cells) {
      settled[activePiece.y + dy][activePiece.x + dx] = activePiece.type;
    }
    clearLines();
    const next = spawnPiece();
    activePiece = canPlace(next, next.x, next.y) ? next : null;
  }
  render();
}

function render() {
  const board = settled.map((row) => row.slice());
  if (activePiece) {
    for (const [dx, dy] of activePiece.cells) {
      board[activePiece.y + dy][activePiece.x + dx] = activePiece.type;
    }
  }
  const sidebar = [
    `Score: ${score}`,
    `Lines: ${linesCleared}`,
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
  const gravity = setInterval(tick, 700);
  readline.emitKeypressEvents(process.stdin);
  process.stdin.setRawMode(true);
  process.stdin.resume();

  function cleanup() {
    clearInterval(gravity);
    process.stdin.setRawMode(false);
    process.stdin.pause();
  }

  process.stdin.on('keypress', (character, key) => {
    if (key.ctrl && key.name === 'c' || key.name === 'q') {
      cleanup();
      process.stdout.write('\n');
      process.exit(0);
    }
    if (!activePiece) return;
    if (key.name === 'up') {
      if (rotatePiece(activePiece)) render();
      return;
    }
    const dx = key.name === 'left' ? -1 : key.name === 'right' ? 1 : 0;
    if (dx && canPlace(activePiece, activePiece.x + dx, activePiece.y)) {
      activePiece.x += dx;
      render();
    } else if (key.name === 'down') {
      tick();
    }
  });
  process.on('exit', cleanup);
}

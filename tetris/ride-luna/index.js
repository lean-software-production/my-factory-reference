#!/usr/bin/env node
'use strict';

const clear = '\x1b[2J\x1b[H';
const reset = '\x1b[0m';

const BOARD_WIDTH = 10;
const BOARD_HEIGHT = 20;
const SHAPES = {
  I: [[1, 1, 1, 1]],
  O: [[1, 1], [1, 1]],
  T: [[0, 1, 0], [1, 1, 1]],
  S: [[0, 1, 1], [1, 1, 0]],
  Z: [[1, 1, 0], [0, 1, 1]],
  J: [[1, 0, 0], [1, 1, 1]],
  L: [[0, 0, 1], [1, 1, 1]],
};

function spawnPiece(type = Object.keys(SHAPES)[Math.floor(Math.random() * 7)]) {
  const shape = SHAPES[type];
  if (!shape) throw new Error(`Unknown tetromino: ${type}`);
  return {
    type,
    shape: shape.map((row) => row.slice()),
    x: Math.floor((BOARD_WIDTH - shape[0].length) / 2),
    y: 0,
  };
}

function render(board = Array.from({ length: BOARD_HEIGHT }, () => Array(BOARD_WIDTH).fill(false)), piece = null) {
  const border = `+${'-'.repeat(BOARD_WIDTH * 2)}+`;
  const cells = board.map((row) => row.slice());
  if (piece) {
    piece.shape.forEach((row, dy) => row.forEach((cell, dx) => {
      const x = piece.x + dx;
      const y = piece.y + dy;
      if (cell && y >= 0 && y < BOARD_HEIGHT && x >= 0 && x < BOARD_WIDTH) cells[y][x] = true;
    }));
  }
  const rows = cells.map((row) => `|${row.map((cell) => cell ? '[]' : '  ').join('')}|`);
  process.stdout.write(`${clear}TETRIS\n${border}\n${rows.join('\n')}\n${border}\n`);
}

function start() {
  const piece = spawnPiece();
  render(undefined, piece);
  if (!process.stdin.isTTY) {
    process.stdout.write('(Run in a terminal to play.)\n');
    return;
  }

  process.stdin.setRawMode(true);
  process.stdin.resume();
  process.stdin.setEncoding('utf8');
  const cleanup = () => {
    process.stdin.setRawMode(false);
    process.stdin.pause();
    process.stdout.write(`${reset}\n`);
  };
  process.stdin.on('data', (key) => {
    if (key === 'q' || key === '\u0003') {
      cleanup();
      process.exit(0);
    }
  });
}

if (require.main === module) start();

module.exports = { BOARD_WIDTH, BOARD_HEIGHT, SHAPES, spawnPiece, render, start };

#!/usr/bin/env node
'use strict';

const clear = '\x1b[2J\x1b[H';
const reset = '\x1b[0m';

const BOARD_WIDTH = 10;
const BOARD_HEIGHT = 20;

function render(board = Array.from({ length: BOARD_HEIGHT }, () => Array(BOARD_WIDTH).fill(false))) {
  const border = `+${'-'.repeat(BOARD_WIDTH * 2)}+`;
  const rows = board.map((row) => `|${row.map((cell) => cell ? '[]' : '  ').join('')}|`);
  process.stdout.write(`${clear}TETRIS\n${border}\n${rows.join('\n')}\n${border}\n`);
}

function start() {
  render();
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

module.exports = { BOARD_WIDTH, BOARD_HEIGHT, render, start };

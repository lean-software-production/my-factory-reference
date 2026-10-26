#!/usr/bin/env node
'use strict';

const WIDTH = 10;
const HEIGHT = 20;

function emptyBoard() {
  return Array.from({ length: HEIGHT }, () => Array(WIDTH).fill(0));
}

function renderBoard(board = emptyBoard()) {
  const rows = ['┌' + '─'.repeat(WIDTH * 2) + '┐'];
  for (let y = 0; y < HEIGHT; y++) {
    const cells = Array.from({ length: WIDTH }, (_, x) => board[y]?.[x] ? '██' : '  ');
    rows.push('│' + cells.join('') + '│');
  }
  rows.push('└' + '─'.repeat(WIDTH * 2) + '┘');
  return rows.join('\n');
}

function initialScreen() {
  return [renderBoard(), 'Terminal Tetris', 'Press any key to start · Q to quit'].join('\n');
}

function render() {
  process.stdout.write('\x1b[2J\x1b[H' + initialScreen() + '\n');
}

function main() {
  render();
  if (!process.stdin.isTTY) return;
  process.stdin.setRawMode(true);
  process.stdin.resume();
  const onKey = (key) => {
    if (key === '\u0003' || key.toLowerCase() === 'q') {
      process.stdin.setRawMode(false);
      process.stdin.pause();
      process.exit(0);
    }
    render();
  };
  process.stdin.on('data', onKey);
}

if (require.main === module) main();
module.exports = { WIDTH, HEIGHT, emptyBoard, renderBoard, initialScreen };

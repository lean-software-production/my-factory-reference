#!/usr/bin/env node

'use strict';

process.stdout.write('TETRIS\n\nWelcome to terminal Tetris!\nPress Q or Ctrl+C to quit.\n');

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

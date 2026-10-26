#!/usr/bin/env node
'use strict';

const clear = '\x1b[2J\x1b[H';
const reset = '\x1b[0m';

function render() {
  process.stdout.write(`${clear}TETRIS\n\nTerminal Tetris\n\nPress q to quit.\n`);
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

module.exports = { render, start };

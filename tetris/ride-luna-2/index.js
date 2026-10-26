#!/usr/bin/env node
'use strict';

const WIDTH = 10;
const HEIGHT = 20;

function initialScreen() {
  const rows = [
    '┌────────────┐',
    '│            │',
    '│            │',
    '│            │',
    '│            │',
    '│            │',
    '│            │',
    '│            │',
    '│            │',
    '│            │',
    '│            │',
    '│            │',
    '│            │',
    '│            │',
    '│            │',
    '│            │',
    '│            │',
    '│            │',
    '│            │',
    '│            │',
    '│            │',
    '└────────────┘',
    'Terminal Tetris',
    'Press any key to start · Q to quit',
  ];
  return rows.join('\n');
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
module.exports = { WIDTH, HEIGHT, initialScreen };

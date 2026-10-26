#!/usr/bin/env node
'use strict';

const WIDTH = 10;
const HEIGHT = 20;
const PIECES = {
  I: [[1, 1, 1, 1]],
  O: [[1, 1], [1, 1]],
  T: [[0, 1, 0], [1, 1, 1]],
  S: [[0, 1, 1], [1, 1, 0]],
  Z: [[1, 1, 0], [0, 1, 1]],
  J: [[1, 0, 0], [1, 1, 1]],
  L: [[0, 0, 1], [1, 1, 1]],
};

function emptyBoard() {
  return Array.from({ length: HEIGHT }, () => Array(WIDTH).fill(0));
}

function spawnPiece(type, board = emptyBoard()) {
  const shape = PIECES[type];
  if (!shape) throw new Error(`Unknown tetromino: ${type}`);
  const piece = { type, shape: shape.map(row => row.slice()), x: Math.floor((WIDTH - shape[0].length) / 2), y: 0 };
  return collides(board, piece) ? null : piece;
}

function collides(board, piece, dx = 0, dy = 0) {
  for (let y = 0; y < piece.shape.length; y++) {
    for (let x = 0; x < piece.shape[y].length; x++) {
      if (!piece.shape[y][x]) continue;
      const bx = piece.x + x + dx;
      const by = piece.y + y + dy;
      if (bx < 0 || bx >= WIDTH || by >= HEIGHT || (by >= 0 && board[by][bx])) return true;
    }
  }
  return false;
}

function movePiece(board, piece, dx, dy) {
  if (collides(board, piece, dx, dy)) return false;
  piece.x += dx;
  piece.y += dy;
  return true;
}

function rotatePiece(board, piece, direction = 1) {
  const height = piece.shape.length;
  const width = piece.shape[0].length;
  const rotated = Array.from({ length: width }, (_, y) =>
    Array.from({ length: height }, (_, x) =>
      direction >= 0 ? piece.shape[height - 1 - x][y] : piece.shape[x][width - 1 - y]
    )
  );
  const candidate = { ...piece, shape: rotated };
  if (collides(board, candidate)) return false;
  piece.shape = rotated;
  return true;
}

function softDrop(board, piece) {
  return movePiece(board, piece, 0, 1);
}

function hardDrop(board, piece) {
  let distance = 0;
  while (softDrop(board, piece)) distance++;
  return distance;
}

function lockPiece(board, piece) {
  for (let y = 0; y < piece.shape.length; y++) {
    for (let x = 0; x < piece.shape[y].length; x++) {
      const by = piece.y + y;
      const bx = piece.x + x;
      if (piece.shape[y][x] && by >= 0 && by < HEIGHT && bx >= 0 && bx < WIDTH) {
        board[by][bx] = piece.type;
      }
    }
  }
}

function clearCompletedRows(board) {
  const remaining = board.filter(row => !row.every(Boolean));
  const cleared = HEIGHT - remaining.length;
  while (remaining.length < HEIGHT) remaining.unshift(Array(WIDTH).fill(0));
  board.splice(0, HEIGHT, ...remaining);
  return cleared;
}

function tick(state) {
  if (!state.active || state.gameOver) return;
  if (!softDrop(state.board, state.piece)) {
    lockPiece(state.board, state.piece);
    clearCompletedRows(state.board);
    state.piece = spawnPiece(state.nextType, state.board);
    state.nextType = Object.keys(PIECES)[Math.floor(Math.random() * Object.keys(PIECES).length)];
    if (!state.piece) state.gameOver = true;
  }
}

function renderBoard(board = emptyBoard(), piece = null) {
  const rows = ['┌' + '─'.repeat(WIDTH * 2) + '┐'];
  for (let y = 0; y < HEIGHT; y++) {
    const cells = Array.from({ length: WIDTH }, (_, x) => {
      const occupied = board[y]?.[x] || (piece && y >= piece.y && y < piece.y + piece.shape.length && x >= piece.x && x < piece.x + piece.shape[y - piece.y].length && piece.shape[y - piece.y][x]);
      return occupied ? '██' : '  ';
    });
    rows.push('│' + cells.join('') + '│');
  }
  rows.push('└' + '─'.repeat(WIDTH * 2) + '┘');
  return rows.join('\n');
}

function initialScreen() {
  return [renderBoard(), 'Terminal Tetris', 'Press any key to start · Q to quit'].join('\n');
}

function render(state) {
  const screen = state?.active
    ? renderBoard(state.board, state.piece) + (state.gameOver ? '\nGAME OVER' : '\nTerminal Tetris')
    : initialScreen();
  process.stdout.write('\x1b[2J\x1b[H' + screen + '\n');
}

function main() {
  const state = { board: emptyBoard(), piece: null, nextType: 'T', active: false, gameOver: false };
  render(state);
  if (!process.stdin.isTTY) return;
  process.stdin.setRawMode(true);
  process.stdin.resume();
  const timer = setInterval(() => { tick(state); render(state); }, 700);
  const onKey = (key) => {
    if (key === '\u0003' || key.toLowerCase() === 'q') {
      clearInterval(timer);
      process.stdin.setRawMode(false);
      process.stdin.pause();
      process.exit(0);
    }
    if (!state.active) {
      state.active = true;
      state.piece = spawnPiece(state.nextType, state.board);
      state.nextType = Object.keys(PIECES)[Math.floor(Math.random() * Object.keys(PIECES).length)];
    }
    render(state);
  };
  process.stdin.on('data', onKey);
}

if (require.main === module) main();
module.exports = { WIDTH, HEIGHT, PIECES, emptyBoard, spawnPiece, collides, movePiece, rotatePiece, softDrop, hardDrop, lockPiece, clearCompletedRows, tick, renderBoard, initialScreen };

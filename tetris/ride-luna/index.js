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

function canPlace(board, piece, x = piece.x, y = piece.y) {
  return piece.shape.every((row, dy) => row.every((cell, dx) => {
    if (!cell) return true;
    const boardX = x + dx;
    const boardY = y + dy;
    return boardX >= 0 && boardX < BOARD_WIDTH && boardY >= 0 && boardY < BOARD_HEIGHT && !board[boardY][boardX];
  }));
}

function movePiece(board, piece, dx, dy) {
  if (!canPlace(board, piece, piece.x + dx, piece.y + dy)) return false;
  piece.x += dx;
  piece.y += dy;
  return true;
}

function clearCompletedLines(board) {
  let cleared = 0;
  for (let y = board.length - 1; y >= 0; y--) {
    if (board[y].every(Boolean)) {
      board.splice(y, 1);
      board.unshift(Array(BOARD_WIDTH).fill(false));
      cleared++;
      y++;
    }
  }
  return cleared;
}

function rotatePiece(board, piece) {
  const rotated = piece.shape[0].map((_, x) =>
    piece.shape.map((row) => row[x]).reverse());
  // Try the rotation in place, then small horizontal/vertical kicks near walls
  // and the floor. A failed rotation leaves the piece untouched.
  const kicks = [[0, 0], [-1, 0], [1, 0], [-2, 0], [2, 0], [0, -1], [-1, -1], [1, -1]];
  for (const [dx, dy] of kicks) {
    if (canPlace(board, { ...piece, shape: rotated }, piece.x + dx, piece.y + dy)) {
      piece.shape = rotated;
      piece.x += dx;
      piece.y += dy;
      return true;
    }
  }
  return false;
}

function render(board = Array.from({ length: BOARD_HEIGHT }, () => Array(BOARD_WIDTH).fill(false)), piece = null, score = 0, gameOver = false) {
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
  process.stdout.write(`${clear}TETRIS  Score: ${score}\n${border}\n${rows.join('\n')}\n${border}\n${gameOver ? 'GAME OVER - Press r to restart or q to quit' : ''}\n`);
}

function start() {
  let board = Array.from({ length: BOARD_HEIGHT }, () => Array(BOARD_WIDTH).fill(false));
  let piece = spawnPiece();
  let score = 0;
  let gameOver = !canPlace(board, piece);
  render(board, piece, score, gameOver);
  if (!process.stdin.isTTY) {
    process.stdout.write('(Run in a terminal to play.)\n');
    return;
  }

  process.stdin.setRawMode(true);
  process.stdin.resume();
  const lockPiece = () => {
    piece.shape.forEach((row, dy) => row.forEach((cell, dx) => {
      if (cell) board[piece.y + dy][piece.x + dx] = true;
    }));
    const lines = clearCompletedLines(board);
    score += [0, 100, 300, 500, 800][lines] || lines * 200;
    piece = spawnPiece();
    if (!canPlace(board, piece)) {
      gameOver = true;
      clearInterval(fallTimer);
    }
  };
  const fallDelay = () => Math.max(100, 800 - Math.floor(score / 500) * 80);
  const fall = () => {
    if (gameOver) return;
    if (!movePiece(board, piece, 0, 1)) lockPiece();
    render(board, piece, score, gameOver);
    clearInterval(fallTimer);
    if (!gameOver) fallTimer = setInterval(fall, fallDelay());
  };
  let fallTimer = setInterval(fall, fallDelay());
  process.stdin.setEncoding('utf8');
  const cleanup = () => {
    clearInterval(fallTimer);
    process.stdin.setRawMode(false);
    process.stdin.pause();
    process.stdout.write(`${reset}\n`);
  };
  process.stdin.on('data', (key) => {
    if (key === 'q' || key === '\u0003') {
      cleanup();
      process.exit(0);
    }
    if (gameOver) {
      if (key === 'r') {
        board = Array.from({ length: BOARD_HEIGHT }, () => Array(BOARD_WIDTH).fill(false));
        score = 0;
        piece = spawnPiece();
        gameOver = !canPlace(board, piece);
        if (!gameOver) fallTimer = setInterval(fall, fallDelay());
        render(board, piece, score, gameOver);
      }
      return;
    }
    if (key === ' ') {
      while (movePiece(board, piece, 0, 1)) { /* drop to the landing position */ }
      lockPiece();
      clearInterval(fallTimer);
      fallTimer = setInterval(fall, fallDelay());
      render(board, piece, score);
      return;
    }
    if (key === 'w' || key === '\u001b[A') {
      rotatePiece(board, piece);
      render(board, piece, score);
      return;
    }
    const movement = { '\u001b[D': [-1, 0], a: [-1, 0], '\u001b[C': [1, 0], d: [1, 0], '\u001b[B': [0, 1], s: [0, 1] }[key];
    if (movement) {
      movePiece(board, piece, movement[0], movement[1]);
      render(board, piece, score);
      if (movement[1] && score) {
        clearInterval(fallTimer);
        fallTimer = setInterval(fall, fallDelay());
      }
    }
  });
}

if (require.main === module) start();

module.exports = { BOARD_WIDTH, BOARD_HEIGHT, SHAPES, spawnPiece, canPlace, movePiece, clearCompletedLines, rotatePiece, render, start };

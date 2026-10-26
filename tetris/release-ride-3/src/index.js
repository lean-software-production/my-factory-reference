const readline = require('node:readline');
const { render, WIDTH } = require('./render');
const { spawnPiece } = require('./pieces');
const { createBoard, canPlace, move, rotate, hardDrop, step, clearLines } = require('./game');

const input = process.stdin;
const output = process.stdout;

if (!input.isTTY || !output.isTTY) {
  console.error('Tetris needs an interactive terminal.');
  process.exitCode = 1;
} else {
  let closed = false;
  let timer;
  let screenOpen = false;

  function close() {
    if (closed) return;
    closed = true;
    clearInterval(timer);
    if (input.isRaw) input.setRawMode(false);
    input.pause();
    if (screenOpen) output.write('\x1b[?25h\x1b[?1049l'); // Show cursor and leave alternate screen.
  }

  function quit() {
    close();
    process.exit();
  }

  process.once('SIGINT', quit);
  process.once('SIGTERM', quit);
  process.once('SIGHUP', quit);
  process.once('exit', close);

  readline.emitKeypressEvents(input);
  input.setRawMode(true);
  input.resume();
  screenOpen = true;
  output.write('\x1b[?1049h\x1b[?25l\x1b[H\x1b[2J'); // Alternate screen, hide cursor.
  let board;
  let active;
  let score;
  let lines;
  let gameOver;
  function restart() {
    clearInterval(timer);
    board = createBoard();
    active = spawnPiece(WIDTH);
    score = 0;
    lines = 0;
    gameOver = false;
    draw();
    timer = setInterval(fall, 700);
  }
  function draw() {
    output.write('\x1b[H' + render(board, { active, score, lines, gameOver }) + '\x1b[K');
  }
  function afterLock() {
    const cleared = clearLines(board);
    lines += cleared;
    score += 100 * cleared * cleared;
    const next = spawnPiece(WIDTH);
    if (canPlace(board, next)) {
      active = next;
    } else {
      active = null;
      gameOver = true;
      clearInterval(timer);
    }
  }
  function fall() {
    if (gameOver) return;
    active = step(board, active);
    if (!active) afterLock();
    draw();
  }
  input.on('keypress', (_character, key) => {
    if (key?.name === 'q' || (key?.ctrl && key.name === 'c')) return quit();
    if (key?.name === 'r') return restart();
    if (gameOver) return;
    if (key?.name === 'left') move(board, active, -1);
    else if (key?.name === 'right') move(board, active, 1);
    else if (key?.name === 'down') return fall();
    else if (key?.name === 'up') rotate(board, active);
    else if (key?.name === 'space') {
      active = hardDrop(board, active);
      afterLock();
    } else return;
    draw();
  });

  restart();
}

const readline = require('node:readline');
const { render } = require('./render');
const { createGame, move, rotate, tick } = require('./game');

function start() {
  if (!process.stdin.isTTY || !process.stdout.isTTY) {
    console.error('Tetris needs an interactive terminal.');
    process.exitCode = 1;
    return;
  }

  const input = process.stdin;
  const output = process.stdout;
  let closed = false;
  let timer;
  const game = createGame();

  function draw() {
    output.write(`\x1b[H${render(game.board, game.score, game.gameOver ? null : game.piece, game.gameOver)}`);
  }

  function close() {
    if (closed) return;
    closed = true;
    clearInterval(timer);
    input.removeListener('keypress', onKey);
    process.removeListener('SIGINT', close);
    input.setRawMode(false);
    input.pause();
    output.write('\x1b[?25h\x1b[0m\n');
  }

  function onKey(_text, key) {
    if (!key) return;
    if (key.name === 'q' || (key.ctrl && key.name === 'c')) {
      close();
      return;
    }
    if (game.gameOver) return;
    if (key.name === 'left' || key.name === 'a') move(game, -1);
    else if (key.name === 'right' || key.name === 'd') move(game, 1);
    else if (key.name === 'down' || key.name === 's') tick(game);
    else if (key.name === 'up' || key.name === 'w') rotate(game);
    else return;
    draw();
    if (game.gameOver) clearInterval(timer);
  }

  readline.emitKeypressEvents(input);
  input.setRawMode(true);
  input.resume();
  input.on('keypress', onKey);
  process.on('SIGINT', close);
  output.write('\x1b[?25l\x1b[2J');
  draw();
  timer = setInterval(() => {
    tick(game);
    draw();
    if (game.gameOver) clearInterval(timer);
  }, 700);
}

start();

const readline = require('node:readline');
const { render } = require('./render');
const { createGame, move, rotate, tick, hardDrop } = require('./game');

let game = createGame();
const interactive = process.stdin.isTTY && process.stdout.isTTY;
function draw() {
  if (interactive) process.stdout.write('\x1b[H\x1b[J');
  process.stdout.write(render(game.board, game.score, game.piece, game.gameOver) + '\n');
}

if (interactive) {
  const wasRaw = process.stdin.isRaw;
  let cleaned = false;
  function cleanup() {
    if (cleaned) return;
    cleaned = true;
    process.stdin.setRawMode(Boolean(wasRaw));
    process.stdin.pause();
    process.stdout.write('\x1b[?25h\x1b[?1049l');
  }
  process.on('exit', cleanup);
  process.on('SIGINT', () => process.exit(130));
  process.on('SIGTERM', () => process.exit(143));
  process.stdout.write('\x1b[?1049h\x1b[?25l');
  readline.emitKeypressEvents(process.stdin);
  process.stdin.setRawMode(true);
  process.stdin.resume();
  draw();
  process.stdin.on('keypress', (_, key) => {
    if ((key.ctrl && key.name === 'c') || key.name === 'q') {
      process.exit();
      return;
    }
    if (key.name === 'r') {
      game = createGame();
      draw();
      return;
    }
    if (game.gameOver) return;
    let changed = false;
    switch (key.name) {
      case 'left': changed = move(game, -1, 0); break;
      case 'right': changed = move(game, 1, 0); break;
      case 'up': changed = rotate(game); break;
      case 'down': tick(game); changed = true; break;
      case 'space': changed = hardDrop(game); break;
    }
    if (changed) draw();
  });
  setInterval(() => {
    if (!game.gameOver) {
      tick(game);
      draw();
    }
  }, 750);
} else {
  draw();
}

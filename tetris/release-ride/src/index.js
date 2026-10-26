const readline = require('node:readline');
const { render } = require('./render');
const { createGame, move, rotate, tick, hardDrop } = require('./game');

const game = createGame();
function draw() {
  if (process.stdout.isTTY) process.stdout.write('\x1b[H');
  process.stdout.write(render(game.board, game.score, game.piece) + '\n');
}

if (process.stdout.isTTY) process.stdout.write('\x1b[2J');
draw();
if (process.stdin.isTTY && process.stdout.isTTY) {
  readline.emitKeypressEvents(process.stdin);
  process.stdin.setRawMode(true);
  process.stdin.resume();
  process.on('exit', () => process.stdin.setRawMode(false));
  process.stdin.on('keypress', (_, key) => {
    if (key.ctrl && key.name === 'c') {
      process.exit();
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
}

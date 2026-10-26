const { render } = require('./render');
const { createGame, tick } = require('./game');

const game = createGame();
function draw() {
  if (process.stdout.isTTY) process.stdout.write('\x1b[H');
  process.stdout.write(render(game.board, game.score, game.piece) + '\n');
}

if (process.stdout.isTTY) process.stdout.write('\x1b[2J');
draw();
if (process.stdin.isTTY && process.stdout.isTTY) {
  setInterval(() => {
    if (!game.gameOver) {
      tick(game);
      draw();
    }
  }, 750);
}

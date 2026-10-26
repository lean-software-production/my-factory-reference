const { TetrisGame } = require('./game');
const { createTerminal } = require('./terminal');
const { renderGame } = require('./render');

function runGame({ game = new TetrisGame(), input = process.stdin, output = process.stdout,
  intervalMs = 500 } = {}) {
  let timer;
  let stopped = false;
  const terminal = createTerminal({ input, output, onKey });

  function stop() {
    if (stopped) return;
    stopped = true;
    clearInterval(timer);
    process.removeListener('SIGINT', stop);
    process.removeListener('SIGTERM', stop);
    terminal.stop();
  }

  function draw() {
    terminal.draw(renderGame(game));
  }

  function onKey(character, key) {
    if (key?.name === 'q' || (key?.ctrl && key?.name === 'c')) {
      stop();
      return;
    }
    if (game.gameOver) return;
    switch (key?.name) {
      case 'left': game.move(-1); break;
      case 'right': game.move(1); break;
      case 'up': game.rotate(); break;
      case 'down': game.softDrop(); break;
      case 'space': game.hardDrop(); break;
      default:
        if (character !== ' ') return;
        game.hardDrop();
    }
    draw();
    if (game.gameOver) clearInterval(timer);
  }

  try {
    terminal.start();
    draw();
    if (!game.gameOver) {
      timer = setInterval(() => {
        game.step();
        draw();
        if (game.gameOver) clearInterval(timer);
      }, intervalMs);
    }
    process.on('SIGINT', stop);
    process.on('SIGTERM', stop);
  } catch (error) {
    stop();
    throw error;
  }
  return { stop };
}

if (require.main === module) {
  try {
    runGame();
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}

module.exports = { runGame };

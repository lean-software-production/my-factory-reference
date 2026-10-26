const readline = require('node:readline');

const MAX_ROWS = 24;

// Own the terminal for the lifetime of the game. Call stop() to restore it.
function createTerminal({ input = process.stdin, output = process.stdout, onKey }) {
  let running = false;
  let wasRaw = false;

  function handleKeypress(character, key) {
    onKey(character, key);
  }

  return {
    start() {
      if (running) return;
      if (!input.isTTY || !output.isTTY || typeof input.setRawMode !== 'function') {
        throw new Error('Tetris needs an interactive terminal.');
      }
      running = true;
      wasRaw = input.isRaw === true;
      readline.emitKeypressEvents(input);
      input.setRawMode(true);
      input.resume();
      input.on('keypress', handleKeypress);
      output.write('\x1b[?1049h\x1b[?25l'); // Alternate screen, hidden cursor.
    },
    draw(lines) {
      if (!running) throw new Error('Terminal is not started.');
      if (!Array.isArray(lines) || lines.length > MAX_ROWS ||
          lines.some((line) => typeof line !== 'string' || line.includes('\n') || line.includes('\r'))) {
        throw new Error(`Display must contain at most ${MAX_ROWS} single-line strings.`);
      }
      output.write(`\x1b[H\x1b[J${lines.join('\n')}`);
    },
    stop() {
      if (!running) return;
      running = false;
      input.removeListener('keypress', handleKeypress);
      input.setRawMode(wasRaw);
      input.pause();
      output.write('\x1b[?25h\x1b[?1049l'); // Restore cursor and original screen.
    },
  };
}

module.exports = { createTerminal, MAX_ROWS };

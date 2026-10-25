const { MAX_ROWS } = require('./terminal');

// The 20-row board, its two borders, and two status rows use exactly 24 rows.
function renderGame(game) {
  const border = `+${'-'.repeat(game.width * 2)}+`;
  const board = game.visibleBoard().map((row) =>
    `|${row.map((block) => block === null ? '  ' : '[]').join('')}|`);
  const lines = [
    border,
    ...board,
    border,
    `Score: ${game.score}  Lines: ${game.lines}`,
    game.gameOver
      ? 'GAME OVER - Press Q to quit'
      : 'Left/Right move | Up rotate | Down soft drop | Space hard drop | Q quit',
  ];
  if (lines.length > MAX_ROWS) throw new Error('Game display exceeds 24 terminal rows.');
  return lines;
}

module.exports = { renderGame };

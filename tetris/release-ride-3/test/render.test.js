const test = require('node:test');
const assert = require('node:assert/strict');
const { render, WIDTH, HEIGHT } = require('../src/render');
const { createBoard } = require('../src/game');
const { SHAPES } = require('../src/pieces');

test('the full display fits in 24 rows, even with active piece and game over', () => {
  const board = createBoard();
  board[HEIGHT - 1].fill('I');
  const active = { type: 'T', cells: SHAPES.T, x: 3, y: 0 };
  for (const gameOver of [false, true]) {
    const rows = render(board, { score: 123456, lines: 42, active, gameOver }).split('\r\n');
    assert.equal(rows.length, 24);
    assert.match(rows[0], /Score: 123456.*Lines: 42/);
    assert.equal(rows[1], `+${'-'.repeat(WIDTH * 2)}+`);
    assert.equal(rows[HEIGHT + 2], rows[1]);
    for (const row of rows.slice(2, HEIGHT + 2)) {
      assert.equal(row.length, WIDTH * 2 + 2);
      assert.ok(row.startsWith('|') && row.endsWith('|'));
    }
    assert.match(rows[2], /\[\]/); // Active piece.
    assert.equal(rows[HEIGHT + 1], `|${'[]'.repeat(WIDTH)}|`);
    assert.match(rows[23], gameOver ? /GAME OVER.*Restart.*Quit/ : /Move.*Soft.*Turn.*Hard.*Restart.*Quit/);
  }
});

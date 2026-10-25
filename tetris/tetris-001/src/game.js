// The board stores locked blocks as piece letters; the active piece is separate.
const SHAPES = Object.freeze({
  I: [[0, 1], [1, 1], [2, 1], [3, 1]],
  O: [[1, 0], [2, 0], [1, 1], [2, 1]],
  T: [[1, 0], [0, 1], [1, 1], [2, 1]],
  S: [[1, 0], [2, 0], [0, 1], [1, 1]],
  Z: [[0, 0], [1, 0], [1, 1], [2, 1]],
  J: [[0, 0], [0, 1], [1, 1], [2, 1]],
  L: [[2, 0], [0, 1], [1, 1], [2, 1]],
});
const TYPES = Object.keys(SHAPES);
const LINE_POINTS = [0, 100, 300, 500, 800];

function cells(type, rotation) {
  let result = SHAPES[type].map(([x, y]) => [x, y]);
  if (type === 'O') return result;
  for (let turn = 0; turn < rotation; turn++) {
    result = result.map(([x, y]) => [3 - y, x]);
  }
  return result;
}

class TetrisGame {
  constructor({ width = 10, height = 20, random = Math.random } = {}) {
    if (!Number.isInteger(width) || width < 4 || !Number.isInteger(height) || height < 4) {
      throw new RangeError('Board width and height must be integers of at least 4.');
    }
    this.width = width;
    this.height = height;
    this.random = random;
    this.board = Array.from({ length: height }, () => Array(width).fill(null));
    this.score = 0;
    this.lines = 0;
    this.gameOver = false;
    this.active = null;
    this.spawn();
  }

  // Coordinates of the falling blocks (may be above the top of the board).
  activeCells(piece = this.active) {
    if (!piece) return [];
    return cells(piece.type, piece.rotation).map(([x, y]) => [piece.x + x, piece.y + y]);
  }

  fits(piece) {
    return this.activeCells(piece).every(([x, y]) =>
      x >= 0 && x < this.width && y < this.height &&
      (y < 0 || this.board[y][x] === null));
  }

  spawn() {
    if (this.gameOver) return false;
    const value = this.random();
    if (typeof value !== 'number' || !Number.isFinite(value) || value < 0 || value >= 1) {
      throw new RangeError('random() must return a number from 0 (inclusive) to 1 (exclusive).');
    }
    const type = TYPES[Math.floor(value * TYPES.length)];
    const shape = cells(type, 0);
    this.active = {
      type,
      rotation: 0,
      x: Math.floor((this.width - 4) / 2),
      y: -Math.min(...shape.map(([, y]) => y)),
    };
    if (!this.fits(this.active)) {
      this.active = null;
      this.gameOver = true;
      return false;
    }
    return true;
  }

  move(dx) {
    if (this.gameOver || !this.active || !Number.isInteger(dx)) return false;
    const next = { ...this.active, x: this.active.x + dx };
    if (!this.fits(next)) return false;
    this.active = next;
    return true;
  }

  // Try nearby horizontal positions so rotations beside walls are possible.
  rotate(direction = 1) {
    if (this.gameOver || !this.active || (direction !== 1 && direction !== -1)) return false;
    const rotation = (this.active.rotation + direction + 4) % 4;
    for (const offset of [0, -1, 1, -2, 2]) {
      const next = { ...this.active, rotation, x: this.active.x + offset };
      if (this.fits(next)) {
        this.active = next;
        return true;
      }
    }
    return false;
  }

  step() {
    if (this.gameOver || !this.active) return false;
    const next = { ...this.active, y: this.active.y + 1 };
    if (this.fits(next)) {
      this.active = next;
      return true;
    }
    this.lock();
    return false;
  }

  softDrop() {
    if (this.gameOver || !this.active) return false;
    const before = this.active;
    const moved = this.step();
    if (moved && this.active !== before) this.score += 1;
    return moved;
  }

  hardDrop() {
    if (this.gameOver || !this.active) return 0;
    let distance = 0;
    while (this.fits({ ...this.active, y: this.active.y + 1 })) {
      this.active.y++;
      distance++;
    }
    this.score += distance * 2;
    this.lock();
    return distance;
  }

  lock() {
    if (!this.active || this.gameOver) return;
    const blocks = this.activeCells();
    // A piece that locks above the visible board ends the game.
    if (blocks.some(([, y]) => y < 0)) {
      this.active = null;
      this.gameOver = true;
      return;
    }
    for (const [x, y] of blocks) this.board[y][x] = this.active.type;
    this.active = null;
    const remaining = this.board.filter((row) => row.some((block) => block === null));
    const cleared = this.height - remaining.length;
    this.board = [
      ...Array.from({ length: cleared }, () => Array(this.width).fill(null)),
      ...remaining,
    ];
    this.lines += cleared;
    this.score += LINE_POINTS[cleared] * (Math.floor((this.lines - cleared) / 10) + 1);
    this.spawn();
  }

  // A snapshot for display; mutating it never changes the game state.
  visibleBoard() {
    const display = this.board.map((row) => [...row]);
    for (const [x, y] of this.activeCells()) {
      if (y >= 0) display[y][x] = this.active.type;
    }
    return display;
  }
}

module.exports = { TetrisGame, SHAPES };

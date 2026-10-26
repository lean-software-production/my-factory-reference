# Terminal Tetris

Requires Node.js 18+ and an interactive terminal. Run `npm start` to play.

- **← / →** move the falling piece
- **↑** rotate (with small wall kicks)
- **↓** soft drop one row
- **Space** hard drop and lock
- **R** restart, including after game over
- **Q** or **Ctrl+C** quit

Complete horizontal lines disappear and award 100, 300, 500 or 800 points for clearing 1–4 lines at once. The game ends when a new piece cannot spawn. The display uses 22 terminal rows; a terminal at least 24 rows tall is recommended.

Run `npm test` to check the game rules.

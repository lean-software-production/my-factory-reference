# Terminal Tetris

Requires Node.js 18+ and an interactive terminal. Run `npm start` to play. The entire display occupies 24 rows; use a terminal at least 24 rows tall (and 54 columns wide for the controls).

- **Left / Right:** move the falling piece
- **Down:** soft drop one row
- **Up:** rotate clockwise
- **Space:** hard drop to the bottom
- **R:** restart, including after game over
- **Q** or **Ctrl+C:** quit

Complete horizontal rows to clear them. The score increases by 100 times the square of the number of rows cleared at once. The game ends when a new piece cannot enter the board.

Run `npm test` for board-rule and display-size tests.

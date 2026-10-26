# Terminal Tetris

Run with `npm start` in a terminal. Use a terminal at least 24 rows tall to see the entire game. The game uses no external packages. In a non-interactive shell, it prints one initial screen and exits.

| Key | Action |
| --- | --- |
| Left / Right arrows | Move the falling piece |
| Up arrow | Rotate clockwise (if space permits) |
| Down arrow | Soft drop one step |
| Space | Hard drop and lock immediately |
| R | Restart |
| Q or Ctrl+C | Quit |

Filled rows disappear and earn 100 points each. When a new piece cannot spawn, the game ends; press R to play again.

Run automated board-rule and display-height checks with `npm test`.

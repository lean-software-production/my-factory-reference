# Terminal Tetris

Requires Node.js 18 or newer and an interactive terminal. From this directory, run:

```sh
npm start
```

The game uses a 10×20 board and fits within 24 terminal rows. Use a terminal at least 24 rows tall to see the complete display.

| Key | Action |
| --- | --- |
| Left / Right arrows or A / D | Move |
| Down arrow or S | Soft drop |
| Up arrow or W | Rotate |
| Space | Hard drop |
| R | Restart |
| Q or Ctrl+C | Quit |

Run automated board-rule and display-height checks with `npm test`.

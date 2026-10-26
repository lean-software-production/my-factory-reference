# The sketchbook kit, vendored

These files are copied, not fetched, so the replay page needs no network.

- `sketchbook.css` and `sketchbook.js` come from `kit/` in
  `lean-software-production/brand`, at commit `9f95426`. `sketchbook.js` is
  unchanged. In `sketchbook.css` the one change is the first `@import` (the
  Google Fonts stylesheet), which is replaced by three `@font-face` rules
  that point at the fonts in `fonts/` beside it.
- `fonts/` comes from `concepts/software-factory/fonts/` in the same
  repository and commit: `luckiest-guy.ttf`, `patrick-hand.ttf` and
  `patrick-hand-sc.ttf`, each with its licence file. Patrick Hand and
  Patrick Hand SC are under the SIL Open Font License 1.1
  (`OFL-Patrick-Hand.txt`, `OFL-Patrick-Hand-SC.txt`). Luckiest Guy is under
  the Apache License 2.0 (`LICENSE-Luckiest-Guy.txt`).

// Package replayweb holds the page that replays a run in a browser: its
// HTML, styles and scripts, and the sketchbook kit they draw with, all
// embedded in the factory so that showing a run needs no network and nothing
// installed.
package replayweb

import (
	"embed"
	"io/fs"
)

//go:embed web
var web embed.FS

// Files is the page's files, as the page asks for them: index.html at its
// root, the kit in kit/.
func Files() fs.FS {
	files, err := fs.Sub(web, "web")
	if err != nil {
		panic(err) // web is embedded above, so it is there
	}
	return files
}

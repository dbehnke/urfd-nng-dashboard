package assets

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var staticAssets embed.FS

func GetAssets() fs.FS {
	f, _ := fs.Sub(staticAssets, "dist")
	return f
}

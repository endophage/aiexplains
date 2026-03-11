//go:build embed

package cmd

import (
	"embed"
	"io/fs"
)

//go:embed all:frontend
var frontendRawFS embed.FS

func embeddedFrontend() (fs.FS, bool) {
	sub, err := fs.Sub(frontendRawFS, "frontend")
	if err != nil {
		return nil, false
	}
	return sub, true
}

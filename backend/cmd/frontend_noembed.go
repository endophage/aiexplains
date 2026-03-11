//go:build !embed

package cmd

import "io/fs"

func embeddedFrontend() (fs.FS, bool) {
	return nil, false
}

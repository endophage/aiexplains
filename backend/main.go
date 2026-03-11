package main

import (
	"os"
	"strings"

	"github.com/endophage/aiexplains/backend/cmd"
)

func main() {
	// When launched as a macOS .app bundle, inject serve --webview automatically.
	// A shell script CFBundleExecutable doesn't properly initialize the Cocoa run
	// loop, so the binary must be the direct CFBundleExecutable and detect the
	// bundle context itself.
	exe, _ := os.Executable()
	if strings.Contains(exe, ".app/Contents/MacOS/") {
		os.Args = []string{os.Args[0], "serve", "--webview"}
	}

	cmd.Execute()
}

package web

import (
	"embed"
	"io/fs"
	"os"
)

//go:embed static/*
var embeddedFiles embed.FS

// StaticFS returns the embedded web/static filesystem rooted at static/.
func StaticFS() (fs.FS, error) {
	return fs.Sub(embeddedFiles, "static")
}

// LicenseBytes returns the GNU AGPL v3.0 license content, reading from the
// workspace root LICENSE file when available and falling back to the embedded copy.
func LicenseBytes() []byte {
	if data, err := os.ReadFile("LICENSE"); err == nil && len(data) > 0 {
		return data
	}
	if data, err := embeddedFiles.ReadFile("static/LICENSE"); err == nil && len(data) > 0 {
		return data
	}
	return []byte("GNU AFFERO GENERAL PUBLIC LICENSE Version 3, 19 November 2007\nSee https://www.gnu.org/licenses/agpl-3.0.txt\n")
}

package azure

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed templates/*.bicep
var templateFS embed.FS

// WriteTemplate extracts an embedded Bicep template to a temp directory
// and returns the path. The caller should clean up the dir when done.
func WriteTemplate(name string) (string, func(), error) {
	data, err := templateFS.ReadFile("templates/" + name)
	if err != nil {
		return "", nil, fmt.Errorf("embedded template %q not found: %w", name, err)
	}

	tmpDir, err := os.MkdirTemp("", "devlake-bicep-*")
	if err != nil {
		return "", nil, err
	}

	path := filepath.Join(tmpDir, name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		os.RemoveAll(tmpDir)
		return "", nil, err
	}

	cleanup := func() { os.RemoveAll(tmpDir) }
	return path, cleanup, nil
}

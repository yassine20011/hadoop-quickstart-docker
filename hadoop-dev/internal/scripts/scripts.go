package scripts

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed shared/*
var embeddedFiles embed.FS

// ExtractTo ensures that the essential cluster startup scripts are extracted to the given work directory.
func ExtractTo(workDir string) error {
	sharedDir := filepath.Join(workDir, "shared")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		return err
	}

	files, err := fs.ReadDir(embeddedFiles, "shared")
	if err != nil {
		return err
	}

	for _, file := range files {
		targetPath := filepath.Join(sharedDir, file.Name())
		content, err := embeddedFiles.ReadFile("shared/" + file.Name())
		if err != nil {
			return err
		}
		
		// Normalize CRLF to LF for bash scripts to run correctly in Linux containers
		content = []byte(strings.ReplaceAll(string(content), "\r\n", "\n"))

		if err := os.WriteFile(targetPath, content, 0o755); err != nil {
			return err
		}
	}
	return nil
}

package config

import (
	"os"
	"path/filepath"
)

// Resolve workspace directory
func ResolveWorkDir(customDir string) (string, error) {
	if customDir != "" {
		return filepath.Abs(customDir)
	}

	if env := os.Getenv("HADOOP_DEV_HOME"); env != "" {
		return filepath.Abs(env)
	}

	cwd, err := os.Getwd()
	if err == nil {
		if hasFile(filepath.Join(cwd, "hadoop.env")) || hasDir(filepath.Join(cwd, "shared")) {
			return cwd, nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Abs(".")
	}
	return filepath.Join(home, ".hadoop-dev"), nil
}

func hasFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func hasDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

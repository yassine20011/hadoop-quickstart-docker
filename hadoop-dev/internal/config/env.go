package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ParseHadoopEnv reads a hadoop.env file and returns a Docker-compatible
// KEY=VALUE slice, skipping blank lines and comments.
func ParseHadoopEnv(path string) ([]string, error) {
	all, err := ParseHadoopEnvRaw(path)
	if err != nil {
		return nil, err
	}
	var envs []string
	for _, line := range all {
		if strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		envs = append(envs, line)
	}
	return envs, nil
}

// ParseHadoopEnvRaw returns every non-empty line including comments.
// Used by the config editor to round-trip the file without losing comments.
func ParseHadoopEnvRaw(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), " \t")
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

package config

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed default.env
var DefaultHadoopEnv []byte

func ParseHadoopEnv(path string) ([]string, error) {
	all, err := ParseHadoopEnvRaw(path)
	if err != nil {
		return nil, err
	}
	return filterEnvLines(all), nil
}

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

func parseRawBytes(data []byte) []string {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), " \t")
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func filterEnvLines(lines []string) []string {
	var envs []string
	for _, line := range lines {
		if strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		envs = append(envs, line)
	}
	return envs
}

func EnsureHadoopEnv(workDir string) ([]string, bool, error) {
	envPath := filepath.Join(workDir, "hadoop.env")
	created := false

	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		if err := os.WriteFile(envPath, DefaultHadoopEnv, 0o644); err == nil {
			created = true
		} else {
			return filterEnvLines(parseRawBytes(DefaultHadoopEnv)), false, nil
		}
	}

	envs, err := ParseHadoopEnv(envPath)
	if err != nil {
		return filterEnvLines(parseRawBytes(DefaultHadoopEnv)), false, nil
	}
	return envs, created, nil
}

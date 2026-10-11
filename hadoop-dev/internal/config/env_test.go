package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureHadoopEnv_CreatesFile(t *testing.T) {
	dir := t.TempDir()

	envs, created, err := EnsureHadoopEnv(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Fatalf("expected created to be true")
	}

	content, err := os.ReadFile(filepath.Join(dir, "hadoop.env"))
	if err != nil {
		t.Fatalf("failed to read created file: %v", err)
	}
	if !strings.Contains(string(content), "HDFS_CONF_dfs_replication=1") {
		t.Fatalf("expected file to contain default replication setting")
	}
	if !strings.Contains(string(content), "HADOOP_OPTS=-XX:+TieredCompilation") {
		t.Fatalf("expected file to contain HADOOP_OPTS")
	}

	foundReplication := false
	for _, env := range envs {
		if env == "HDFS_CONF_dfs_replication=1" {
			foundReplication = true
			break
		}
	}
	if !foundReplication {
		t.Fatalf("expected envs to contain HDFS_CONF_dfs_replication=1")
	}
}

func TestEnsureHadoopEnv_PreservesExisting(t *testing.T) {
	dir := t.TempDir()
	customContent := "HDFS_CONF_dfs_replication=5\nCUSTOM_VAR=hello\n"
	if err := os.WriteFile(filepath.Join(dir, "hadoop.env"), []byte(customContent), 0o644); err != nil {
		t.Fatalf("failed to write custom file: %v", err)
	}

	envs, created, err := EnsureHadoopEnv(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Fatalf("expected created to be false for existing file")
	}

	foundCustom := false
	for _, env := range envs {
		if env == "CUSTOM_VAR=hello" {
			foundCustom = true
			break
		}
	}
	if !foundCustom {
		t.Fatalf("expected custom var to be loaded")
	}
}

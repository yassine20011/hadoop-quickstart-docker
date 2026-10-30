package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWorkDir_ExplicitFlag(t *testing.T) {
	temp := t.TempDir()
	res, err := ResolveWorkDir(temp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected, _ := filepath.Abs(temp)
	if res != expected {
		t.Fatalf("expected %s, got %s", expected, res)
	}
}

func TestResolveWorkDir_EnvVar(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("HADOOP_DEV_HOME", temp)

	res, err := ResolveWorkDir("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected, _ := filepath.Abs(temp)
	if res != expected {
		t.Fatalf("expected %s, got %s", expected, res)
	}
}

func TestResolveWorkDir_CwdWithHadoopEnv(t *testing.T) {
	t.Setenv("HADOOP_DEV_HOME", "")
	temp := t.TempDir()
	if err := os.WriteFile(filepath.Join(temp, "hadoop.env"), []byte("TEST=1\n"), 0o644); err != nil {
		t.Fatalf("failed to write test env: %v", err)
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	_ = os.Chdir(temp)

	res, err := ResolveWorkDir("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected, _ := filepath.Abs(temp)
	if res != expected {
		t.Fatalf("expected %s, got %s", expected, res)
	}
}

func TestResolveWorkDir_DefaultHome(t *testing.T) {
	t.Setenv("HADOOP_DEV_HOME", "")
	temp := t.TempDir()

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	_ = os.Chdir(temp)

	res, err := ResolveWorkDir("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".hadoop-dev")
	if res != expected {
		t.Fatalf("expected %s, got %s", expected, res)
	}
}

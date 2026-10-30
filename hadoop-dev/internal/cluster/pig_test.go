package cluster

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestPigDirAndIsPigInstalled(t *testing.T) {
	temp := t.TempDir()
	if IsPigInstalled(temp) {
		t.Fatalf("expected Pig not to be installed in empty temp dir")
	}

	pigBin := filepath.Join(PigDir(temp), "bin", "pig")
	if err := os.MkdirAll(filepath.Dir(pigBin), 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(pigBin, []byte("#!/bin/bash\n"), 0o755); err != nil {
		t.Fatalf("failed to write pig bin: %v", err)
	}

	if !IsPigInstalled(temp) {
		t.Fatalf("expected Pig to be reported as installed")
	}
}

func TestExtractTarGz(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	content := []byte("hello pig")
	hdr := &tar.Header{
		Name: "test-dir/hello.txt",
		Mode: 0o644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed to write header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("failed to write content: %v", err)
	}
	_ = tw.Close()
	_ = gw.Close()

	temp := t.TempDir()
	archivePath := filepath.Join(temp, "test.tar.gz")
	if err := os.WriteFile(archivePath, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("failed to write archive: %v", err)
	}

	destDir := filepath.Join(temp, "extracted")
	if err := extractTarGz(archivePath, destDir); err != nil {
		t.Fatalf("failed to extract: %v", err)
	}

	extractedFile := filepath.Join(destDir, "test-dir", "hello.txt")
	got, err := os.ReadFile(extractedFile)
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("expected %s, got %s", string(content), string(got))
	}
}

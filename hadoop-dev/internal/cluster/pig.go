package cluster

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hadoop-dev/internal/output"
)

const PigURL = "https://archive.apache.org/dist/pig/pig-0.17.0/pig-0.17.0.tar.gz"

func PigDir(workDir string) string {
	return filepath.Join(workDir, "shared", "pig-0.17.0")
}

func IsPigInstalled(workDir string) bool {
	binPath := filepath.Join(PigDir(workDir), "bin", "pig")
	info, err := os.Stat(binPath)
	return err == nil && !info.IsDir()
}

func InstallPig(workDir string, force bool, pr *output.Printer) error {
	sharedDir := filepath.Join(workDir, "shared")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		return fmt.Errorf("create shared dir: %w", err)
	}

	pigPath := PigDir(workDir)
	if !force && IsPigInstalled(workDir) {
		if pr != nil {
			pr.Warn("Apache Pig 0.17.0 is already installed at " + pigPath)
			pr.Sub("Run 'hadoop-dev pig' to start the Grunt shell.")
		}
		return nil
	}

	if force {
		_ = os.RemoveAll(pigPath)
	}

	label := "Downloading Apache Pig 0.17.0"
	if pr != nil {
		pr.Begin(label)
	}

	resp, err := http.Get(PigURL)
	if err != nil {
		if pr != nil {
			pr.Fail(label)
		}
		return fmt.Errorf("download pig: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if pr != nil {
			pr.Fail(label)
		}
		return fmt.Errorf("download pig failed: HTTP %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "pig-0.17.0-*.tar.gz")
	if err != nil {
		if pr != nil {
			pr.Fail(label)
		}
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	pw := &progressWriter{
		writer: tmpFile,
		total:  resp.ContentLength,
		pr:     pr,
		label:  label,
	}

	if _, err := io.Copy(pw, resp.Body); err != nil {
		tmpFile.Close()
		if pr != nil {
			pr.Fail(label)
		}
		return fmt.Errorf("save download: %w", err)
	}
	tmpFile.Close()

	if pr != nil {
		pr.Done(label, "220MB")
	}

	extractLabel := "Extracting Apache Pig 0.17.0"
	if pr != nil {
		pr.Begin(extractLabel)
	}

	if err := extractTarGz(tmpName, sharedDir); err != nil {
		if pr != nil {
			pr.Fail(extractLabel)
		}
		return fmt.Errorf("extract pig: %w", err)
	}

	// Set execution permissions on bin
	binDir := filepath.Join(pigPath, "bin")
	if entries, err := os.ReadDir(binDir); err == nil {
		for _, e := range entries {
			_ = os.Chmod(filepath.Join(binDir, e.Name()), 0o755)
		}
	}

	if pr != nil {
		pr.Done(extractLabel, "")
	}

	return nil
}

type progressWriter struct {
	writer    io.Writer
	total     int64
	written   int64
	lastPrint time.Time
	pr        *output.Printer
	label     string
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.writer.Write(b)
	p.written += int64(n)

	if p.pr != nil && (time.Since(p.lastPrint) > 250*time.Millisecond) {
		p.lastPrint = time.Now()
		mbWritten := float64(p.written) / (1024 * 1024)
		if p.total > 0 {
			mbTotal := float64(p.total) / (1024 * 1024)
			p.pr.Progress(p.label, fmt.Sprintf("%.1f / %.1f MB", mbWritten, mbTotal))
		} else {
			p.pr.Progress(p.label, fmt.Sprintf("%.1f MB", mbWritten))
		}
	}
	return n, err
}

func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	cleanDest := filepath.Clean(destDir)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, hdr.Name)
		if !strings.HasPrefix(filepath.Clean(target), cleanDest) {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(hdr.Mode)
			if mode == 0 {
				mode = 0o644
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		case tar.TypeSymlink:
			_ = os.Remove(target)
			_ = os.Symlink(hdr.Linkname, target)
		}
	}

	return nil
}

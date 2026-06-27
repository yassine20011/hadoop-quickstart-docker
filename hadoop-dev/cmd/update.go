package cmd

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"hadoop-dev/internal/output"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update hadoop-dev to the latest version",
	Example: `  hadoop-dev update
  hadoop-dev update -v`,
	RunE: func(cmd *cobra.Command, args []string) error {
		pr := output.New(verbose, noColor)

		pr.Begin("Checking for updates")
		latest, assetURL, err := fetchLatestReleaseInfo()
		if err != nil {
			pr.Fail("Checking for updates")
			return err
		}
		pr.Done("Checking for updates", "latest is "+latest)

		if latest == "v"+Version || Version == "dev" {
			pr.Warn("Already up to date (" + latest + ")")
			if Version == "dev" {
				pr.Sub("Note: running a dev build, but continuing with update anyway.")
			} else {
				return nil
			}
		}

		exePath, err := os.Executable()
		if err != nil {
			return err
		}

		pr.Begin("Downloading update")
		tmpExe, err := downloadAndExtract(assetURL)
		if err != nil {
			pr.Fail("Downloading update")
			return err
		}
		defer os.Remove(tmpExe)
		pr.Done("Downloading update", "")

		pr.Begin("Applying update")
		if err := replaceBinary(exePath, tmpExe); err != nil {
			pr.Fail("Applying update")
			return err
		}
		pr.Done("Applying update", "")

		pr.Header(fmt.Sprintf("Successfully updated to %s!", latest))
		return nil
	},
}

func fetchLatestReleaseInfo() (string, string, error) {
	resp, err := http.Get("https://api.github.com/repos/yassine20011/hadoop-quickstart-docker/releases/latest")
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", err
	}

	expectedArch := runtime.GOARCH
	if expectedArch == "amd64" {
		expectedArch = "x86_64" // Sometimes release pipelines use this mapping
	}
	// Goreleaser uses exact GOARCH (amd64/arm64)
	expectedArch = runtime.GOARCH

	expectedSuffix := fmt.Sprintf("%s_%s.tar.gz", runtime.GOOS, expectedArch)
	if runtime.GOOS == "windows" {
		expectedSuffix = fmt.Sprintf("%s_%s.zip", runtime.GOOS, expectedArch)
	}

	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, expectedSuffix) {
			return release.TagName, asset.BrowserDownloadURL, nil
		}
	}

	return "", "", fmt.Errorf("no suitable asset found for %s/%s in release %s", runtime.GOOS, runtime.GOARCH, release.TagName)
}

func downloadAndExtract(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	tmpFile, err := os.CreateTemp("", "hadoop-dev-update-")
	if err != nil {
		return "", err
	}
	tmpName := tmpFile.Name()

	if strings.HasSuffix(url, ".zip") {
		// Extract zip (Windows)
		r, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if err != nil {
			tmpFile.Close()
			return "", err
		}
		for _, f := range r.File {
			if strings.HasSuffix(f.Name, "hadoop-dev.exe") {
				rc, err := f.Open()
				if err != nil {
					tmpFile.Close()
					return "", err
				}
				_, err = io.Copy(tmpFile, rc)
				rc.Close()
				tmpFile.Close()
				return tmpName, err
			}
		}
	} else {
		// Extract tar.gz (Unix)
		gr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			tmpFile.Close()
			return "", err
		}
		tr := tar.NewReader(gr)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				tmpFile.Close()
				return "", err
			}
			if hdr.Name == "hadoop-dev" {
				_, err = io.Copy(tmpFile, tr)
				tmpFile.Close()
				os.Chmod(tmpName, 0755)
				return tmpName, err
			}
		}
	}

	tmpFile.Close()
	return "", fmt.Errorf("binary not found in archive")
}

func replaceBinary(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// Handle "file in use" on Windows by renaming first
	if runtime.GOOS == "windows" {
		oldPath := dst + ".old"
		os.Remove(oldPath) // Best effort
		if err := os.Rename(dst, oldPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	} else {
		os.Remove(dst)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

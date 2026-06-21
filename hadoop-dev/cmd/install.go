package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install hadoop-dev so it can be run from anywhere",
	RunE: func(cmd *cobra.Command, args []string) error {
		exePath, err := os.Executable()
		if err != nil {
			return err
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		var binDir string
		var targetName string

		if runtime.GOOS == "windows" {
			binDir = filepath.Join(home, "AppData", "Local", "hadoop-dev")
			targetName = "hadoop-dev.exe"
		} else {
			// On Linux/macOS, ~/.local/bin is a standard user-local bin directory
			binDir = filepath.Join(home, ".local", "bin")
			targetName = "hadoop-dev"
		}

		if err := os.MkdirAll(binDir, 0o755); err != nil {
			return fmt.Errorf("create install directory: %w", err)
		}

		targetPath := filepath.Join(binDir, targetName)

		// Don't copy if it's already running from the target path
		if strings.EqualFold(exePath, targetPath) {
			fmt.Println("✅ hadoop-dev is already installed at", targetPath)
			return nil
		}

		fmt.Printf("📦 Copying executable to %s...\n", targetPath)
		if err := copyFile(exePath, targetPath); err != nil {
			return fmt.Errorf("copy executable: %w", err)
		}

		// Ensure it is executable on Unix
		if runtime.GOOS != "windows" {
			_ = os.Chmod(targetPath, 0o755)
		}

		// Update PATH if necessary
		if runtime.GOOS == "windows" {
			fmt.Println("🔧 Adding to Windows User PATH...")
			psScript := fmt.Sprintf(`
$Path = [Environment]::GetEnvironmentVariable("PATH", "User")
$Target = "%s"
if ($Path -notmatch [regex]::Escape($Target)) {
    $NewPath = $Path + ";" + $Target
    [Environment]::SetEnvironmentVariable("PATH", $NewPath, "User")
    Write-Host "Added to PATH."
} else {
    Write-Host "Already in PATH."
}
`, binDir)
			
			cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
			out, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Printf("⚠️ Could not automatically update PATH: %v\n%s\n", err, out)
				fmt.Printf("Please manually add %s to your system PATH.\n", binDir)
			} else {
				fmt.Println("✅ Successfully installed!")
				fmt.Println("You may need to restart your terminal (or open a new PowerShell window) for the new PATH to take effect.")
			}
		} else {
			fmt.Println("✅ Successfully installed to", targetPath)
			fmt.Println("Make sure", binDir, "is in your system PATH (e.g., in ~/.bashrc or ~/.zshrc).")
		}

		return nil
	},
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// If destination exists, we should try to remove it first in case it's currently running
	_ = os.Remove(dst)

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func init() {
	rootCmd.AddCommand(installCmd)
}

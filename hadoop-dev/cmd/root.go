package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var workDir string

var rootCmd = &cobra.Command{
	Use:   "hadoop-dev",
	Short: "Cross-platform Hadoop development cluster manager",
	Long: `hadoop-dev spins up a local Hadoop development cluster using Docker.

No WSL, no docker-compose, no Bash required.
The only prerequisite is a running Docker daemon.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(
		&workDir, "work-dir", ".",
		"project directory containing shared/ and hadoop.env",
	)
}

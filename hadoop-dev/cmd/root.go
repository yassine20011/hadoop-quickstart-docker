package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	workDir string
	noColor bool
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "hadoop-dev",
	Short: "Cross-platform Hadoop development cluster manager",
	Long: `hadoop-dev spins up a local Hadoop development cluster using Docker.

No WSL, no docker-compose, no Bash required.
The only prerequisite is a running Docker daemon.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return rootCmd.ExecuteContext(ctx)
}

func init() {
	rootCmd.PersistentFlags().StringVar(
		&workDir, "work-dir", ".",
		"project directory containing shared/ and hadoop.env",
	)
	rootCmd.PersistentFlags().BoolVar(
		&noColor, "no-color", false,
		"disable colored output",
	)
	rootCmd.PersistentFlags().BoolVarP(
		&verbose, "verbose", "v", false,
		"show detailed per-step output",
	)
}

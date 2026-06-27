package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version variables — set at build time via goreleaser ldflags:
//
//	-X hadoop-dev/cmd.Version={{.Version}}
//	-X hadoop-dev/cmd.Commit={{.Commit}}
//	-X hadoop-dev/cmd.Date={{.Date}}
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Example: `  hadoop-dev version
  hadoop-dev version --no-color`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "hadoop-dev %s\n", Version)
		fmt.Fprintf(cmd.OutOrStdout(), "  commit: %s\n", Commit)
		fmt.Fprintf(cmd.OutOrStdout(), "  built:  %s\n", Date)
		fmt.Fprintf(cmd.OutOrStdout(), "  go:     %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

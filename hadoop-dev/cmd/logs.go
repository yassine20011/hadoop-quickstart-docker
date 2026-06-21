package cmd

import (
	"context"
	"fmt"

	"hadoop-dev/internal/cluster"

	"github.com/spf13/cobra"
)

var follow bool

var logsCmd = &cobra.Command{
	Use:   "logs [service]",
	Short: "Fetch logs from a cluster container",
	Args:  cobra.MaximumNArgs(1),
	Example: `  hadoop-dev logs
  hadoop-dev logs namenode
  hadoop-dev logs -f hive-server`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		service := "namenode"
		if len(args) > 0 {
			service = args[0]
		}

		mgr, err := cluster.NewManager()
		if err != nil {
			return fmt.Errorf("cannot connect to Docker: %w", err)
		}
		defer mgr.Close()

		return mgr.Logs(ctx, service, follow)
	},
}

func init() {
	logsCmd.Flags().BoolVarP(&follow, "follow", "f", false, "stream logs in real time")
	rootCmd.AddCommand(logsCmd)
}

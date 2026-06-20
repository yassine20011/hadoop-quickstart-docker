package cmd

import (
	"context"
	"fmt"

	"hadoop-dev/internal/cluster"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop and remove all cluster containers",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		mgr, err := cluster.NewManager()
		if err != nil {
			return fmt.Errorf("cannot connect to Docker: %w", err)
		}
		defer mgr.Close()

		return mgr.Stop(ctx)
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

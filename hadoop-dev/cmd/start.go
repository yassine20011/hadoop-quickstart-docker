package cmd

import (
	"context"
	"fmt"

	"hadoop-dev/internal/cluster"

	"github.com/spf13/cobra"
)

var (
	preset   string
	dnCount  int
	noAttach bool
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Hadoop cluster",
	Example: `  hadoop-dev start
  hadoop-dev start --preset standard --datanodes 3
  hadoop-dev start --preset full --no-attach`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		p, err := cluster.ParsePreset(preset)
		if err != nil {
			return err
		}

		cfg := cluster.Config{
			WorkDir:  workDir,
			Preset:   p,
			DNCount:  dnCount,
			NoAttach: noAttach,
		}

		mgr, err := cluster.NewManager()
		if err != nil {
			return fmt.Errorf("cannot connect to Docker: %w\n\nIs Docker running?", err)
		}
		defer mgr.Close()

		return mgr.Start(ctx, cfg)
	},
}

func init() {
	startCmd.Flags().StringVar(&preset, "preset", "minimal",
		"cluster preset: minimal (Hadoop only), standard (+ Hive), full (+ Hive + HBase)")
	startCmd.Flags().IntVar(&dnCount, "datanodes", 2,
		"number of DataNodes to start (>= 1)")
	startCmd.Flags().BoolVar(&noAttach, "no-attach", false,
		"skip attaching to an interactive NameNode shell after startup")
	rootCmd.AddCommand(startCmd)
}

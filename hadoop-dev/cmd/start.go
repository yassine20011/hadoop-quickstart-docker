package cmd

import (
	"fmt"

	"hadoop-dev/internal/cluster"
	"hadoop-dev/internal/output"

	"github.com/spf13/cobra"
)

type startOptions struct {
	preset   string
	dnCount  int
	noAttach bool
}

var startOpts startOptions

var startCmd = &cobra.Command{
	Use:   "start [flags]",
	Short: "Start the Hadoop cluster",
	Example: `  hadoop-dev start
  hadoop-dev start --preset standard --datanodes 3
  hadoop-dev start --preset full --no-attach
  hadoop-dev start -v`,
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := cluster.ParsePreset(startOpts.preset)
		if err != nil {
			return err
		}

		cfg := cluster.Config{
			WorkDir:  workDir,
			Preset:   p,
			DNCount:  startOpts.dnCount,
			NoAttach: startOpts.noAttach,
		}

		baseMgr, err := cluster.NewManager()
		if err != nil {
			return fmt.Errorf("cannot connect to Docker: %w\n\nIs Docker running?", err)
		}
		defer baseMgr.Close()

		pr := output.New(verbose, noColor)
		mgr := baseMgr.WithPrinter(pr)

		return mgr.Start(cmd.Context(), cfg)
	},
}

func init() {
	startCmd.Flags().StringVar(&startOpts.preset, "preset", "minimal",
		"cluster preset: minimal (Hadoop only), standard (+ Hive), full (+ Hive + HBase)")
	startCmd.Flags().IntVar(&startOpts.dnCount, "datanodes", 2,
		"number of DataNodes to start (>= 1)")
	startCmd.Flags().BoolVar(&startOpts.noAttach, "no-attach", false,
		"skip attaching to an interactive NameNode shell after startup")
	rootCmd.AddCommand(startCmd)
}

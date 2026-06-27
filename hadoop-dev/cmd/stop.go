package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"hadoop-dev/internal/cluster"
	"hadoop-dev/internal/output"

	"github.com/spf13/cobra"
)

type stopOptions struct {
	force bool
}

var stopOpts stopOptions

var stopCmd = &cobra.Command{
	Use:   "stop [flags]",
	Short: "Stop and remove all cluster containers",
	Example: `  hadoop-dev stop
  hadoop-dev stop --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !stopOpts.force {
			fmt.Fprintln(cmd.ErrOrStderr(), "WARNING! This will stop and remove all cluster containers and networks.")
			fmt.Fprint(cmd.ErrOrStderr(), "Are you sure you want to continue? [y/N] ")

			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
				return nil
			}
		}

		mgr, err := cluster.NewManager()
		if err != nil {
			return fmt.Errorf("cannot connect to Docker: %w", err)
		}
		defer mgr.Close()

		pr := output.New(verbose, noColor)
		pr.Begin("Stopping cluster")
		if err := mgr.Stop(cmd.Context()); err != nil {
			pr.Fail("Stopping cluster")
			return err
		}
		pr.Done("Stopping cluster", "")
		return nil
	},
}

func init() {
	stopCmd.Flags().BoolVarP(&stopOpts.force, "force", "f", false, "skip confirmation prompt")
	rootCmd.AddCommand(stopCmd)
}

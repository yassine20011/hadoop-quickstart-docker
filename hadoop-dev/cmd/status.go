package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"hadoop-dev/internal/cluster"

	"github.com/spf13/cobra"
)

type statusOptions struct {
	quiet  bool
	format string
}

var statusOpts statusOptions

var statusCmd = &cobra.Command{
	Use:     "status [flags]",
	Short:   "Show the status of all cluster containers",
	Aliases: []string{"ps"},
	Example: `  hadoop-dev status
  hadoop-dev status -q
  hadoop-dev status --format '{{.Name}}: {{.Status}}'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if statusOpts.quiet && statusOpts.format != "" {
			return fmt.Errorf("--quiet and --format cannot be used together")
		}

		mgr, err := cluster.NewManager()
		if err != nil {
			return fmt.Errorf("cannot connect to Docker: %w", err)
		}
		defer mgr.Close()

		containers, err := mgr.ListContainers(cmd.Context())
		if err != nil {
			return err
		}

		if len(containers) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No cluster containers found. Run 'hadoop-dev start' to begin.")
			return nil
		}

		if statusOpts.quiet {
			for _, c := range containers {
				name := strings.TrimPrefix(c.Names[0], "/")
				fmt.Fprintln(cmd.OutOrStdout(), name)
			}
			return nil
		}

		out := cmd.OutOrStdout()
		w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tSTATUS\tIMAGE")
		for _, c := range containers {
			name := strings.TrimPrefix(c.Names[0], "/")
			status := cluster.ColorStatus(c.State, c.Status, noColor)
			fmt.Fprintf(w, "%s\t%s\t%s\n", name, status, c.Image)
		}
		return w.Flush()
	},
}

func init() {
	statusCmd.Flags().BoolVarP(&statusOpts.quiet, "quiet", "q", false, "only display container names")
	statusCmd.Flags().StringVar(&statusOpts.format, "format", "", "format output using a Go template (e.g. '{{.Name}}: {{.Status}}')")
	rootCmd.AddCommand(statusCmd)
}

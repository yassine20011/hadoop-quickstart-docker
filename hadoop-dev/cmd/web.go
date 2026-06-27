package cmd

import (
	"fmt"

	"hadoop-dev/internal/web"

	"github.com/spf13/cobra"
)

type webOptions struct {
	port int
}

var webOpts webOptions

var webCmd = &cobra.Command{
	Use:   "web [flags]",
	Short: "Launch the interactive web dashboard",
	Long:  `Starts a local web server and opens the Hadoop cluster dashboard in your browser.`,
	Example: `  hadoop-dev web
  hadoop-dev web --port 9090`,
	RunE: func(cmd *cobra.Command, args []string) error {
		srv, err := web.NewServer(workDir, webOpts.port)
		if err != nil {
			return fmt.Errorf("init server: %w", err)
		}
		return srv.Run(cmd.Context())
	},
}

func init() {
	webCmd.Flags().IntVar(&webOpts.port, "port", 8080, "port for the web dashboard")
	rootCmd.AddCommand(webCmd)
}

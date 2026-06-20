package cmd

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"hadoop-dev/internal/web"

	"github.com/spf13/cobra"
)

var webPort int

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Launch the interactive web dashboard",
	Long:  `Starts a local web server and opens the Hadoop cluster dashboard in your browser.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		srv, err := web.NewServer(workDir, webPort)
		if err != nil {
			return fmt.Errorf("init server: %w", err)
		}
		return srv.Run(ctx)
	},
}

func init() {
	webCmd.Flags().IntVar(&webPort, "port", 8080, "port for the web dashboard")
	rootCmd.AddCommand(webCmd)
}

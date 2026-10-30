package cmd

import (
	"fmt"

	"hadoop-dev/internal/cluster"
	"hadoop-dev/internal/output"

	"github.com/spf13/cobra"
)

var forcePigInstall bool

var pigCmd = &cobra.Command{
	Use:   "pig [flags/args...]",
	Short: "Run Apache Pig in the Hadoop cluster",
	Long: `Run Apache Pig inside the NameNode container.

If no arguments are provided, it starts an interactive Grunt shell.
Arguments and scripts are passed directly to the pig executable.`,
	Example: `  hadoop-dev pig
  hadoop-dev pig -x mapreduce
  hadoop-dev pig -f /shared/myscript.pig
  hadoop-dev pig install`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 && args[0] == "install" {
			return runPigInstall(cmd, args[1:])
		}
		if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
			return cmd.Help()
		}

		if !cluster.IsPigInstalled(workDir) {
			return fmt.Errorf("Apache Pig is not installed.\nRun 'hadoop-dev pig install' to install it.")
		}

		mgr, err := cluster.NewManager()
		if err != nil {
			return fmt.Errorf("cannot connect to Docker: %w\n\nIs Docker running?", err)
		}
		defer mgr.Close()

		return mgr.ExecPig(cmd.Context(), args)
	},
}

var pigInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Download and install Apache Pig 0.17.0 into the shared directory",
	Example: `  hadoop-dev pig install
  hadoop-dev pig install --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPigInstall(cmd, args)
	},
}

func runPigInstall(cmd *cobra.Command, args []string) error {
	force := forcePigInstall
	for _, arg := range args {
		if arg == "--force" || arg == "-f" {
			force = true
		}
	}

	pr := output.New(verbose, noColor)
	if err := cluster.InstallPig(workDir, force, pr); err != nil {
		return err
	}

	pr.Header("Apache Pig 0.17.0 ready!")
	fmt.Println("  Run 'hadoop-dev pig' to start the interactive Grunt shell.")
	fmt.Println()
	return nil
}

func init() {
	pigInstallCmd.Flags().BoolVarP(&forcePigInstall, "force", "f", false, "force reinstall Apache Pig")
	pigCmd.AddCommand(pigInstallCmd)
	rootCmd.AddCommand(pigCmd)
}

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/teqneers/cronado/internal/config"
	"github.com/teqneers/cronado/internal/context"
	"github.com/teqneers/cronado/internal/logging"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cronado",
	Short: "An application to run cron jobs in docker containers",
	Long:  `cronado let's you manage cron jobs via labels on containers and run certain commands regularly.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Use 'cronado --help' to see available commands.")
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cfg := config.LoadConfig()

		context.AppCtx = &context.AppContext{
			Config: cfg,
		}

		// Initialize logger
		logging.InitializeLogger(cfg)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// Initialize configuration and environment variables
func init() {
	rootCmd.PersistentFlags().String("config", "", "config file (default: ./config.yaml)")

	_ = viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
}

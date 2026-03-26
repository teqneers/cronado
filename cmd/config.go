package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// configCmd represents the 'config' command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Display configuration values",
	Long:  "Displays the current configuration values loaded from the config file or environment variables.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Configuration:")
		fmt.Printf("App Name: %s\n", viper.GetString("app_name"))
		fmt.Printf("Log Level: %s\n", viper.GetString("log_level"))
	},
}

// Register the config command
func init() {
	rootCmd.AddCommand(configCmd)
}

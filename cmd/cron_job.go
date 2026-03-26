package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/teqneers/cronado/internal/context"
	"github.com/teqneers/cronado/internal/domain"
)

var cronJobCmd = &cobra.Command{
	Use:   "cron-job",
	Short: "Manage cron-jobs",
	Long:  "Manage cron-jobs",
}

var cronJobListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all cron-jobs",
	Long:  "List all cron-jobs",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := context.AppCtx.Config

		resp, err := http.Get(fmt.Sprintf("http://%s:%s/api/cron-job", cfg.ServerConfig.Host, cfg.ServerConfig.Port))
		if err != nil {
			log.Fatalf("Failed to fetch cron jobs: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNoContent {
			fmt.Println("No active cron jobs.")
			return
		}

		var jobs []domain.CronListItem
		if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
			log.Fatalf("Failed to decode response: %v", err)
		}

		fmt.Println("Active Container Cron Jobs:")
		for _, job := range jobs {
			containerDisplay := formatContainerDisplay(job.ContainerName, job.ContainerID)
			fmt.Printf("- Container: %s | Entry ID: %d | Schedule: %s | Cmd: %s\n",
				containerDisplay,
				job.CronJob.SchedulerId,
				job.CronJob.Schedule,
				job.CronJob.Command)
		}

	},
}

// formatContainerDisplay returns a user-friendly container display string
func formatContainerDisplay(containerName, containerID string) string {
	shortID := containerID
	if len(containerID) > 12 {
		shortID = containerID[:12]
	}

	if containerName != "" {
		return fmt.Sprintf("%s (%s)", containerName, shortID)
	}
	return shortID
}

// Register the config command
func init() {
	cronJobCmd.AddCommand(cronJobListCmd)
	rootCmd.AddCommand(cronJobCmd)
}

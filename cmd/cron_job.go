package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

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
		var jobs []domain.CronListItem
		if err := fetchAPI("/api/cron-job", &jobs); err != nil {
			log.Fatalf("Failed to fetch cron jobs: %v", err)
		}
		// An older scheduler without this endpoint answers 404; the job list
		// is still useful on its own.
		var issues []domain.ContainerLabelIssue
		if err := fetchAPI("/api/label-issues", &issues); err != nil {
			log.Printf("Failed to fetch label problems: %v", err)
		}

		if len(jobs) == 0 {
			fmt.Println("No active cron jobs.")
		} else {
			fmt.Println("Active Container Cron Jobs:")
			for _, job := range jobs {
				containerDisplay := formatContainerDisplay(job.ContainerName, job.ContainerID)
				fmt.Printf("- Container: %s | Entry ID: %d | Schedule: %s | Cmd: %s\n",
					containerDisplay,
					job.CronJob.SchedulerId,
					job.CronJob.SchedulerSpec(),
					job.CronJob.Command)
			}
		}

		if len(issues) > 0 {
			fmt.Println("\nLabel Problems:")
			for _, issue := range issues {
				fmt.Printf("- Container: %s | %s\n",
					formatContainerDisplay(issue.ContainerName, issue.ContainerID),
					formatLabelIssue(issue))
			}
		}
	},
}

// fetchAPI GETs path from the running scheduler and decodes the JSON
// response into v.
func fetchAPI(path string, v any) error {
	cfg := context.AppCtx.Config
	url := fmt.Sprintf("http://%s%s", net.JoinHostPort(cfg.ServerConfig.Host, cfg.ServerConfig.Port), path)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if cfg.ServerConfig.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.ServerConfig.APIToken)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

// formatLabelIssue describes a label problem in one line.
func formatLabelIssue(issue domain.ContainerLabelIssue) string {
	if issue.JobRejected {
		return fmt.Sprintf("job %s rejected: %s", issue.Job, issue.Message)
	}
	return fmt.Sprintf("label %s ignored: %s", issue.Label, issue.Message)
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

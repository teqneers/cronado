package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
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
		if err := callAPI(http.MethodGet, "/api/cron-job", &jobs); err != nil {
			log.Fatalf("Failed to fetch cron jobs: %v", err)
		}
		// An older scheduler without this endpoint answers 404; the job list
		// is still useful on its own.
		var issues []domain.ContainerLabelIssue
		if err := callAPI(http.MethodGet, "/api/label-issues", &issues); err != nil {
			log.Printf("Failed to fetch label problems: %v", err)
		}

		if len(jobs) == 0 {
			fmt.Println("No active cron jobs.")
		} else {
			fmt.Println("Active Container Cron Jobs:")
			for _, job := range jobs {
				containerDisplay := formatContainerDisplay(job.ContainerName, job.ContainerID)
				paused := ""
				if job.CronJob.Paused {
					paused = " | PAUSED"
				}
				if job.CronJob.Standby {
					paused += " | STANDBY"
				}
				fmt.Printf("- Container: %s | ID: %s | Schedule: %s | Cmd: %s%s\n",
					containerDisplay,
					job.CronJob.ID,
					job.CronJob.SchedulerSpec(),
					job.CronJob.Command,
					paused)
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

// callAPI sends a request to the running scheduler and decodes the JSON
// response into v, if v is not nil.
func callAPI(method, path string, v any) error {
	cfg := context.AppCtx.Config
	endpoint := fmt.Sprintf("http://%s%s", net.JoinHostPort(localHost(cfg.ServerConfig.Host), cfg.ServerConfig.Port), path)
	req, err := http.NewRequest(method, endpoint, nil)
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

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var apiErr struct {
			Error string `json:"error"`
		}
		if json.NewDecoder(resp.Body).Decode(&apiErr) == nil && apiErr.Error != "" {
			return fmt.Errorf("%s: %s", resp.Status, apiErr.Error)
		}
		return fmt.Errorf("%s", resp.Status)
	}
	if v == nil {
		return nil
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
// jobActionCmd returns a command that POSTs to /api/cron-job/<id>/<action>.
func jobActionCmd(action, short, done string) *cobra.Command {
	return &cobra.Command{
		Use:   action + " <job-id>",
		Short: short,
		Long:  short + ". Needs server.api_token. Job IDs are shown by 'cronado cron-job list'.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			jobID := args[0]
			if err := callAPI(http.MethodPost, "/api/cron-job/"+url.PathEscape(jobID)+"/"+action, nil); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to %s %s: %v\n", action, jobID, err)
				os.Exit(1)
			}
			fmt.Printf("%s %s\n", done, jobID)
		},
	}
}

func init() {
	cronJobCmd.AddCommand(cronJobListCmd)
	cronJobCmd.AddCommand(jobActionCmd("run", "Run a cron job now", "Started"))
	cronJobCmd.AddCommand(jobActionCmd("pause", "Pause the scheduled runs of a cron job", "Paused"))
	cronJobCmd.AddCommand(jobActionCmd("resume", "Resume the scheduled runs of a paused cron job", "Resumed"))
	rootCmd.AddCommand(cronJobCmd)
}

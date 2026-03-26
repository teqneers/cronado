package domain

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	cronadoCtx "github.com/teqneers/cronado/internal/context"
	"github.com/teqneers/cronado/internal/util"

	"github.com/containerd/errdefs"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
)

// InitializeAlreadyRunningContainers scans for already running containers and registers their cron jobs
func InitializeAlreadyRunningContainers() {
	runningFilter := filters.NewArgs(filters.Arg("status", "running"))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	containers, err := GetDockerClient().ContainerList(ctx, dockercontainer.ListOptions{
		All:     true,
		Filters: runningFilter,
	})
	if errdefs.IsUnavailable(err) {
		slog.Error("Docker daemon unavailable, cannot list containers", "error", err)
		return
	} else if errdefs.IsDeadlineExceeded(err) {
		slog.Error("Container listing timed out", "error", err)
		return
	} else if err != nil {
		slog.Error("Failed to list running containers", "error", err)
		return
	}

	for _, container := range containers {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		containerObj, err := GetOrCreateContainer(ctx, container.ID)
		cancel()
		if errdefs.IsNotFound(err) {
			slog.Debug("Container no longer exists during initialization", "container", container.ID)
			continue
		} else if err != nil {
			slog.Error("Failed to get container info during initialization", "container", container.ID, "error", err)
			continue
		}
		handleContainer(containerObj)
	}
}

// StartEventListener listens for Docker container events and manages cron jobs accordingly
func StartEventListener(stop chan os.Signal) {
	eventFilter := filters.NewArgs(filters.Arg("type", "container"))
	// Events stream should not timeout - it's a long-running connection
	eventsChan, errChan := GetDockerClient().Events(context.Background(), events.ListOptions{
		Filters: eventFilter,
	})

	for {
		select {
		case event := <-eventsChan:
			handleDockerEvent(event)
		case err := <-errChan:
			if errdefs.IsUnavailable(err) {
				slog.Error("Docker daemon unavailable", "error", err)
				util.HandleDockerDaemonPanic(err)
			} else if errdefs.IsCanceled(err) {
				slog.Info("Docker event stream was canceled", "error", err)
				return
			} else {
				slog.Error("Error receiving Docker events", "error", err)
				util.HandleDockerDaemonPanic(err)
			}
		case <-stop:
			slog.Debug("Received shutdown signal, stopping event listener")
			GetCronJobManager().Shutdown()
			os.Exit(0)
		}
	}
}

// handleDockerEvent processes Docker container events
func handleDockerEvent(event events.Message) {
	if event.Type == "container" {
		containerID := event.Actor.ID

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		containerObj, err := GetOrCreateContainer(ctx, containerID)
		cancel()

		if errdefs.IsNotFound(err) {
			// Container not found, remove cron jobs
			slog.Info("Container not found, removing cron jobs", "container", containerID)
			GetCronJobManager().Remove(containerID)
			return
		} else if errdefs.IsUnavailable(err) {
			// Docker daemon is temporarily unavailable, retry later
			slog.Warn("Docker daemon unavailable, skipping container inspection", "container", containerID, "error", err)
			return
		} else if errdefs.IsDeadlineExceeded(err) {
			// Request timed out
			slog.Warn("Container inspection timed out", "container", containerID, "error", err)
			return
		} else if err != nil {
			slog.Error("Failed to inspect container", "container", containerID, "error", err)
			return
		}

		switch event.Action {
		case events.ActionStart:
			handleContainer(containerObj)
		case events.ActionDie, events.ActionStop, events.ActionDestroy:
			GetCronJobManager().Remove(containerObj.ID)
		}
	}
}

// handleContainer processes a container and registers/updates its cron jobs
func handleContainer(container *Container) {
	ctx := cronadoCtx.AppCtx
	cronLabelPrefix := ctx.Config.CronLabelPrefix

	containerCrons := parseCronsFromContainer(container, cronLabelPrefix)

	if len(containerCrons) == 0 {
		slog.Debug("No relevant labels found for container", "container", container.DisplayName())
		return
	}

	for _, cronJob := range containerCrons {
		// Check if cron job already exists by ID
		if GetCronJobManager().IsRegistered(cronJob.ID) {
			// No changes detected, skip
			slog.Info("No changes detected for cron job", "job_id", cronJob.ID, "container", container.DisplayName(), "cron_job", cronJob.Name)
			continue
		}

		if cronJob.Enabled {
			registerCronJob(container, cronJob)
		} else {
			// Remove the cron job if it is disabled
			if cronJob.SchedulerId != -1 {
				GetCronJobManager().removeJobTyped(cronJob)
			}
		}
	}
}

// parseCronsFromContainer extracts cron job definitions from container labels with improved robustness
func parseCronsFromContainer(container *Container, cronLabelPrefix string) map[string]CronJob {
	// Phase 1: Parse all labels and group by job name
	builders := make(map[string]*CronJobBuilder)

	prefixWithDot := cronLabelPrefix + "."

	for label, labelValue := range container.Labels {
		if !strings.HasPrefix(label, prefixWithDot) {
			continue
		}

		// Remove prefix and split remaining parts
		remaining := strings.TrimPrefix(label, prefixWithDot)
		parts := strings.SplitN(remaining, ".", 2)

		if len(parts) != 2 {
			slog.Warn("Invalid label format, expected format: prefix.jobname.property",
				"label", label,
				"container", container.DisplayName(),
				"expected", cronLabelPrefix+".jobname.property")
			continue
		}

		jobName := parts[0]
		property := parts[1]

		// Validate job name
		if jobName == "" {
			slog.Warn("Empty job name in label", "label", label, "container", container.DisplayName())
			continue
		}

		// Get or create builder for this job
		if _, exists := builders[jobName]; !exists {
			builders[jobName] = NewCronJobBuilder(jobName, container)
		}

		builder := builders[jobName]

		// Set the property
		switch property {
		case "enabled":
			builder.SetEnabled(labelValue)
		case "schedule":
			builder.SetSchedule(labelValue)
		case "cmd":
			builder.SetCommand(labelValue)
		case "user":
			builder.SetUser(labelValue)
		case "timeout":
			builder.SetTimeout(labelValue)
		default:
			slog.Warn("Unknown cron job property",
				"property", property,
				"label", label,
				"container", container.DisplayName(),
				"valid_properties", "enabled,schedule,cmd,user,timeout")
		}
	}

	// Phase 2: Build valid cron jobs
	result := make(map[string]CronJob, len(builders))

	for jobName, builder := range builders {
		cronJob, err := builder.Build()
		if err != nil {
			slog.Error("Failed to build cron job",
				"job", jobName,
				"container", container.DisplayName(),
				"error", err)
			continue
		}

		result[jobName] = *cronJob
		slog.Debug("Successfully parsed cron job",
			"job", jobName,
			"container", container.DisplayName(),
			"enabled", cronJob.Enabled,
			"schedule", cronJob.Schedule,
			"command", cronJob.Command,
			"user", cronJob.User)
	}

	return result
}

// registerCronJob validates and registers a cron job using the context manager
func registerCronJob(container *Container, cronJob CronJob) {
	if !cronJob.IsValid() {
		slog.Error("Invalid cron job", "container", container.DisplayName(), "cron_job", cronJob.Name)
		return
	}

	// Check if the cron job is already registered
	if GetCronJobManager().IsRegistered(cronJob.ID) {
		slog.Info("Cron job already registered", "job_id", cronJob.ID, "container", container.DisplayName(), "cron_job", cronJob.Name)
		return
	}

	GetCronJobManager().addTyped(container, cronJob)
}

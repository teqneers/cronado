package domain

import (
	"context"
	"log/slog"
	"os"
	"regexp"
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
	_ = syncRunningContainers()
}

// syncRunningContainers registers the cron jobs of all running containers and
// returns the IDs of those containers.
func syncRunningContainers() map[string]bool {
	runningFilter := filters.NewArgs(filters.Arg("status", "running"))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	containers, err := GetDockerClient().ContainerList(ctx, dockercontainer.ListOptions{
		All:     true,
		Filters: runningFilter,
	})
	switch {
	case errdefs.IsUnavailable(err):
		slog.Error("Docker daemon unavailable, cannot list containers", "error", err)
		return nil
	case errdefs.IsDeadlineExceeded(err):
		slog.Error("Container listing timed out", "error", err)
		return nil
	case err != nil:
		slog.Error("Failed to list running containers", "error", err)
		return nil
	}

	running := make(map[string]bool, len(containers))
	for _, container := range containers {
		running[container.ID] = true
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
	return running
}

// resyncAfterReconnect brings the registered jobs back in line with Docker
// after events may have been missed while the event stream was down.
func resyncAfterReconnect() {
	running := syncRunningContainers()
	if running == nil {
		return
	}
	removed := make(map[string]bool)
	for _, job := range GetCronJobManager().GetAllTyped() {
		if job.Container == nil || running[job.Container.ID] || removed[job.Container.ID] {
			continue
		}
		slog.Info("Container stopped while event stream was down, removing cron jobs", "container", job.GetContainerDisplayName())
		GetCronJobManager().Remove(job.Container.ID)
		removed[job.Container.ID] = true
	}
}

// Event stream reconnect settings. After maxEventStreamFailures consecutive
// failures the daemon is considered gone and cronado notifies and exits.
const (
	maxEventStreamFailures = 6
	maxReconnectBackoff    = 30 * time.Second
	stableStreamDuration   = 30 * time.Second
)

// StartEventListener listens for Docker container events and manages cron jobs
// accordingly. A broken event stream is reconnected with backoff.
func StartEventListener(stop chan os.Signal) {
	eventFilter := filters.NewArgs(
		filters.Arg("type", "container"),
		filters.Arg("event", string(events.ActionStart)),
		filters.Arg("event", string(events.ActionDie)),
		filters.Arg("event", string(events.ActionStop)),
		filters.Arg("event", string(events.ActionDestroy)),
	)

	failures := 0
	for {
		connectedAt := time.Now()
		err, stopped := listenForEvents(eventFilter, stop)
		if stopped {
			slog.Debug("Received shutdown signal, stopping event listener")
			GetCronJobManager().Shutdown()
			os.Exit(0)
		}
		if errdefs.IsCanceled(err) {
			slog.Info("Docker event stream was canceled", "error", err)
			return
		}

		if time.Since(connectedAt) >= stableStreamDuration {
			failures = 0
		}
		failures++
		if failures >= maxEventStreamFailures {
			util.HandleDockerDaemonPanic(err)
		}

		backoff := min(time.Duration(1<<(failures-1))*time.Second, maxReconnectBackoff)
		slog.Warn("Docker event stream failed, reconnecting", "error", err, "attempt", failures, "backoff", backoff)
		select {
		case <-time.After(backoff):
		case <-stop:
			GetCronJobManager().Shutdown()
			os.Exit(0)
		}
		resyncAfterReconnect()
	}
}

// listenForEvents consumes one event stream until it fails or a stop signal
// arrives. It returns the stream error, or stopped=true on shutdown.
func listenForEvents(eventFilter filters.Args, stop chan os.Signal) (err error, stopped bool) {
	// Events stream should not timeout - it's a long-running connection
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eventsChan, errChan := GetDockerClient().Events(ctx, events.ListOptions{
		Filters: eventFilter,
	})
	eventStreamConnected.Store(true)
	defer eventStreamConnected.Store(false)

	for {
		select {
		case event := <-eventsChan:
			handleDockerEvent(event)
		case err := <-errChan:
			return err, false
		case <-stop:
			return nil, true
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

		switch {
		case errdefs.IsNotFound(err):
			// Container not found, remove cron jobs
			slog.Info("Container not found, removing cron jobs", "container", containerID)
			GetCronJobManager().Remove(containerID)
			return
		case errdefs.IsUnavailable(err):
			// Docker daemon is temporarily unavailable, retry later
			slog.Warn("Docker daemon unavailable, skipping container inspection", "container", containerID, "error", err)
			return
		case errdefs.IsDeadlineExceeded(err):
			// Request timed out
			slog.Warn("Container inspection timed out", "container", containerID, "error", err)
			return
		case err != nil:
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
		} else if cronJob.SchedulerId != -1 {
			// Remove the cron job if it is disabled
			GetCronJobManager().removeJobTyped(cronJob)
		}
	}
}

// validJobName matches alphanumeric names with hyphens and underscores
var validJobName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

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
		if !validJobName.MatchString(jobName) {
			slog.Warn("Invalid job name, must match [a-zA-Z0-9][a-zA-Z0-9_-]*",
				"job_name", jobName, "label", label, "container", container.DisplayName())
			continue
		}

		// Get or create builder for this job
		if _, exists := builders[jobName]; !exists {
			var minScheduleInterval, maxTimeout time.Duration
			if cronadoCtx.AppCtx != nil && cronadoCtx.AppCtx.Config != nil {
				minScheduleInterval = cronadoCtx.AppCtx.Config.MinScheduleInterval
				maxTimeout = cronadoCtx.AppCtx.Config.MaxTimeout
			}
			builders[jobName] = NewCronJobBuilder(jobName, container, minScheduleInterval, maxTimeout)
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
		case "timezone":
			builder.SetTimezone(labelValue)
		case "timeout":
			builder.SetTimeout(labelValue)
		case "ping_url":
			builder.SetPingURL(labelValue)
		default:
			slog.Warn("Unknown cron job property",
				"property", property,
				"label", label,
				"container", container.DisplayName(),
				"valid_properties", "enabled,schedule,cmd,user,timezone,timeout,ping_url")
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
			"schedule", cronJob.SchedulerSpec(),
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

	if err := GetCronJobManager().addTyped(container, cronJob); err != nil {
		slog.Error("Failed to register cron job", "container", container.DisplayName(), "error", err)
	}
}

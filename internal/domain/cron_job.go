package domain

import (
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
)

// DefaultTimeout is the default command execution timeout
const DefaultTimeout = 30 * time.Second

// CronJob status constants
const (
	StatusIdle    = "idle"
	StatusPending = "pending" // waiting out its jitter delay
	StatusRunning = "running"
)

// Values of the replicas label
const (
	ReplicasOnce = "once" // one replica of a Compose service runs the job
	ReplicasAll  = "all"  // every replica runs the job
)

type CronJob struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Container *Container    `json:"-"`
	Enabled   bool          `json:"enabled"`
	Schedule  string        `json:"schedule"`
	Command   string        `json:"command"`
	User      string        `json:"user"`
	Timezone  string        `json:"timezone,omitempty"`
	Timeout   time.Duration `json:"timeout"`
	Jitter    time.Duration `json:"jitter,omitempty"`
	PingURL   string        `json:"ping_url,omitempty"`
	Replicas  string        `json:"replicas,omitempty"`
	// Image makes the job run in a new container from this image instead of
	// in Container, which then only defines the job.
	Image   string   `json:"image,omitempty"`
	Network string   `json:"network,omitempty"`
	Volumes []string `json:"volumes,omitempty"`
	// Env holds KEY=value entries for the new container. Not exposed in the
	// API, since values are often secrets.
	Env []string `json:"-"`
	// Source is where the job is defined: SourceLabel or SourceConfig
	Source string `json:"source,omitempty"`
	Status string `json:"status"`
	// Paused jobs skip their scheduled runs until resumed. Not persisted:
	// a restart of the container or of cronado resumes the job.
	Paused bool `json:"paused"`
	// Standby jobs belong to a Compose replica whose scheduled runs are left
	// to another replica of the same service (see Replicas).
	Standby     bool         `json:"standby,omitempty"`
	SchedulerId cron.EntryID `json:"scheduler_id"`

	// Execution state, updated after every run
	LastRun      *time.Time `json:"last_run,omitempty"`
	LastSuccess  *time.Time `json:"last_success,omitempty"`
	LastResult   string     `json:"last_result,omitempty"` // "success" or "failure"
	LastError    string     `json:"last_error,omitempty"`
	LastDuration float64    `json:"last_duration_seconds,omitempty"`
	NextRun      *time.Time `json:"next_run,omitempty"`
}

// GetShortContainerID returns the short ID of the associated container
func (c *CronJob) GetShortContainerID() string {
	if c.Container != nil {
		return c.Container.ShortID()
	}
	return "unknown"
}

// GetContainerDisplayName returns a human-friendly container identifier
func (c *CronJob) GetContainerDisplayName() string {
	if c.Container != nil {
		return c.Container.DisplayName()
	}
	return c.GetShortContainerID()
}

// SchedulerSpec returns the schedule as passed to the scheduler, with the
// job's time zone applied as a CRON_TZ prefix.
func (c *CronJob) SchedulerSpec() string {
	if c.Timezone == "" {
		return c.Schedule
	}
	return "CRON_TZ=" + c.Timezone + " " + c.Schedule
}

// IsValid checks if the cron job has all required fields set
func (c *CronJob) IsValid() bool {
	if c.Name == "" {
		slog.Error("Cron job name is empty", "container", c.GetShortContainerID())
		return false
	}
	if c.Container == nil || c.Container.ID == "" {
		slog.Error("Cron job container is empty", "container", c.GetShortContainerID(), "cron_job", c.Name)
		return false
	}
	if c.Schedule == "" {
		slog.Error("Cron job schedule is empty", "container", c.GetShortContainerID(), "cron_job", c.Name)
		return false
	}
	if c.Command == "" && c.Image == "" {
		slog.Error("Cron job command is empty", "container", c.GetShortContainerID(), "cron_job", c.Name)
		return false
	}

	return true
}

// IsRunning returns true if the cron job is currently running
func (c *CronJob) IsRunning() bool {
	return c.Status == StatusRunning
}

// IsIdle returns true if the cron job is idle
func (c *CronJob) IsIdle() bool {
	return c.Status == StatusIdle
}

// SetPending sets the cron job status to pending
func (c *CronJob) SetPending() {
	c.Status = StatusPending
}

// SetRunning sets the cron job status to running
func (c *CronJob) SetRunning() {
	c.Status = StatusRunning
}

// SetIdle sets the cron job status to idle
func (c *CronJob) SetIdle() {
	c.Status = StatusIdle
}

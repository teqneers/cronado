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
	StatusRunning = "running"
)

type CronJob struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Container   *Container    `json:"-"`
	Enabled     bool          `json:"enabled"`
	Schedule    string        `json:"schedule"`
	Command     string        `json:"command"`
	User        string        `json:"user"`
	Timeout     time.Duration `json:"timeout"`
	Status      string        `json:"status"`
	SchedulerId cron.EntryID  `json:"scheduler_id"`
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
	if c.Command == "" {
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

// SetRunning sets the cron job status to running
func (c *CronJob) SetRunning() {
	c.Status = StatusRunning
}

// SetIdle sets the cron job status to idle
func (c *CronJob) SetIdle() {
	c.Status = StatusIdle
}

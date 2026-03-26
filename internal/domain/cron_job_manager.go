package domain

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	cronadoCtx "github.com/teqneers/cronado/internal/context"
)

// Scheduler defines the interface for cron scheduling
type Scheduler interface {
	AddFunc(spec string, cmd func()) (cron.EntryID, error)
	Remove(id cron.EntryID)
	Start()
	Stop() context.Context
	Entries() []cron.Entry
}

// CronJobManager manages cron jobs and their scheduling with dependency injection
type CronJobManager struct {
	initialized bool
	mu          sync.RWMutex
	jobs        map[string]*CronJob
	scheduler   Scheduler
	executor    CommandExecutor
	metrics     MetricsCollector
	notifier    NotificationService
}

// CronJobManagerOptions contains configuration options for CronJobManager
type CronJobManagerOptions struct {
	Scheduler   Scheduler
	Executor    CommandExecutor
	Metrics     MetricsCollector
	Notifier    NotificationService
}

// NewCronJobManager creates a new CronJobManager with dependency injection
func NewCronJobManager(opts CronJobManagerOptions) *CronJobManager {
	// Use default implementations if not provided
	if opts.Scheduler == nil {
		opts.Scheduler = &CronSchedulerAdapter{cron.New()}
	}
	if opts.Executor == nil {
		opts.Executor = NewDockerCommandExecutor(GetDockerClient())
	}
	if opts.Metrics == nil {
		opts.Metrics = NewPrometheusMetricsCollector()
	}
	if opts.Notifier == nil {
		opts.Notifier = NewDefaultNotificationService()
	}

	return &CronJobManager{
		initialized: true,
		jobs:        make(map[string]*CronJob),
		scheduler:   opts.Scheduler,
		executor:    opts.Executor,
		metrics:     opts.Metrics,
		notifier:    opts.Notifier,
	}
}

// Add adds a new cron job to the manager (interface{} version for context compatibility)
func (m *CronJobManager) Add(containerIface interface{}, jobIface interface{}) error {
	container, ok := containerIface.(*Container)
	if !ok {
		return fmt.Errorf("invalid container type")
	}
	job, ok := jobIface.(CronJob)
	if !ok {
		return fmt.Errorf("invalid job type")
	}
	return m.addTyped(container, job)
}

// addTyped is the internal strongly-typed add method
func (m *CronJobManager) addTyped(container *Container, job CronJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	jobID := job.ID
	entryID, err := m.scheduler.AddFunc(job.Schedule, func() {
		m.executeJob(container, &job)
	})
	if err != nil {
		slog.Error("Failed to add cron job", "error", err)
		return err
	}

	job.SchedulerId = entryID
	job.Container = container
	job.SetIdle()
	m.jobs[jobID] = &job

	slog.Info("Cron job registered",
		"job_id", jobID,
		"container", container.DisplayName(),
		"cron_job", job.Name,
		"schedule", job.Schedule,
		"command", job.Command,
		"user", job.User)

	m.metrics.IncrementScheduledJobs()
	return nil
}

// executeJob executes a cron job with proper locking and error handling
func (m *CronJobManager) executeJob(container *Container, job *CronJob) {
	// Check if job is already running
	m.mu.Lock()
	currentJob, exists := m.jobs[job.ID]
	if !exists {
		m.mu.Unlock()
		return
	}
	if currentJob.IsRunning() {
		slog.Warn("Skipping cron job execution - already running", "job_id", job.ID)
		m.mu.Unlock()
		return
	}
	currentJob.SetRunning()
	m.mu.Unlock()

	// Execute the command
	startTime := time.Now()
	slog.Info("Executing cron job",
		"job_id", job.ID,
		"container", container.DisplayName(),
		"cron_job", job.Name)

	err := m.executor.ExecuteCommand(container, job.Name, job.Command, job.User, job.Timeout)
	duration := time.Since(startTime).Seconds()

	// Record metrics
	status := "success"
	if err != nil {
		status = "failure"
		slog.Error("Cron job execution failed",
			"job_id", job.ID,
			"container", container.DisplayName(),
			"cron_job", job.Name,
			"error", err)

		// Send notification on failure
		if m.notifier != nil {
			m.notifier.SendJobFailure(container, job.Name, err)
		}
	} else {
		stdout, stderr := m.executor.GetOutput()
		if stdout != "" {
			slog.Debug("Job stdout", "job_id", job.ID, "output", stdout)
		}
		if stderr != "" {
			slog.Debug("Job stderr", "job_id", job.ID, "error", stderr)
		}
	}

	m.metrics.RecordJobExecution(container.ShortID(), job.Name, status)
	m.metrics.RecordJobDuration(container.ShortID(), job.Name, duration)

	// Mark as idle after execution
	m.mu.Lock()
	if currentJob, exists := m.jobs[job.ID]; exists {
		currentJob.SetIdle()
	}
	m.mu.Unlock()
}

// RemoveJob removes a specific cron job from the manager (interface{} version for context compatibility)
func (m *CronJobManager) RemoveJob(jobIface interface{}) {
	job, ok := jobIface.(CronJob)
	if !ok {
		return
	}
	m.removeJobTyped(job)
}

// removeJobTyped is the internal strongly-typed remove method
func (m *CronJobManager) removeJobTyped(job CronJob) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove from scheduler
	m.scheduler.Remove(job.SchedulerId)

	// Remove from map
	if _, exists := m.jobs[job.ID]; exists {
		delete(m.jobs, job.ID)
		m.metrics.DecrementScheduledJobs()
		slog.Info("Removed cron job",
			"job_id", job.ID,
			"container", job.GetShortContainerID(),
			"cron_job", job.Name)
	}
}

// Remove removes all cron jobs for a specific container ID
func (m *CronJobManager) Remove(containerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for jobID, job := range m.jobs {
		if job.Container != nil && job.Container.ID == containerID {
			m.scheduler.Remove(job.SchedulerId)
			containerShortId := containerID
			if len(containerID) >= 12 {
				containerShortId = containerID[:12]
			}
			slog.Info("Removed cron job",
				"job_id", jobID,
				"container", containerShortId,
				"cron_job", job.Name)
			delete(m.jobs, jobID)
			m.metrics.DecrementScheduledJobs()
		}
	}
}

// GetAll retrieves all cron jobs (interface{} version for context compatibility)
func (m *CronJobManager) GetAll() []interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]interface{}, 0, len(m.jobs))
	for _, job := range m.jobs {
		result = append(result, *job)
	}
	return result
}

// GetAllTyped retrieves all cron jobs with strong typing
func (m *CronJobManager) GetAllTyped() []CronJob {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]CronJob, 0, len(m.jobs))
	for _, job := range m.jobs {
		result = append(result, *job)
	}
	return result
}

// IsRegistered checks if a cron job is already registered by job ID
func (m *CronJobManager) IsRegistered(jobID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.jobs[jobID]
	return exists
}

// GetJobByID returns a cron job by its ID
func (m *CronJobManager) GetJobByID(jobID string) (*CronJob, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, exists := m.jobs[jobID]
	if !exists {
		return nil, false
	}
	// Return a copy to avoid race conditions
	jobCopy := *job
	return &jobCopy, true
}

// Start starts the cron scheduler
func (m *CronJobManager) Start() {
	m.scheduler.Start()
}

// Stop stops the cron scheduler
func (m *CronJobManager) Stop() {
	ctx := m.scheduler.Stop()
	if ctx != nil {
		// Wait for stop to complete
		<-ctx.Done()
	}
}

// Shutdown is an alias for Stop
func (m *CronJobManager) Shutdown() {
	m.Stop()
}

// GetJobCount returns the current number of scheduled jobs
func (m *CronJobManager) GetJobCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.jobs)
}

// CronSchedulerAdapter adapts robfig/cron to the Scheduler interface
type CronSchedulerAdapter struct {
	cron *cron.Cron
}

func (a *CronSchedulerAdapter) AddFunc(spec string, cmd func()) (cron.EntryID, error) {
	return a.cron.AddFunc(spec, cmd)
}

func (a *CronSchedulerAdapter) Remove(id cron.EntryID) {
	a.cron.Remove(id)
}

func (a *CronSchedulerAdapter) Start() {
	a.cron.Start()
}

func (a *CronSchedulerAdapter) Stop() context.Context {
	return a.cron.Stop()
}

func (a *CronSchedulerAdapter) Entries() []cron.Entry {
	return a.cron.Entries()
}

// GetCronJobManager returns the global CronJobManager instance from context
func GetCronJobManager() *CronJobManager {
	if cronadoCtx.AppCtx != nil && cronadoCtx.AppCtx.CronJobManager != nil {
		// Direct cast since we removed the adapter
		if manager, ok := cronadoCtx.AppCtx.CronJobManager.(*CronJobManager); ok {
			return manager
		}
	}
	return nil
}


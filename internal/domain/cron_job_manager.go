package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strings"
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
	// labelIssues holds the label problems of running containers, by container ID
	labelIssues map[string][]ContainerLabelIssue
	scheduler   Scheduler
	executor    CommandExecutor
	metrics     MetricsCollector
	notifier    NotificationService

	// stopping is closed by Stop so that runs waiting out their jitter
	// delay do not hold up shutdown.
	stopping chan struct{}
	stopOnce sync.Once
	// jitterDelay picks the delay for a run with the given maximum jitter.
	jitterDelay func(maxJitter time.Duration) time.Duration
	// manualRuns tracks runs started by RunNow, which the scheduler does not
	// know about, so that Stop can wait for them.
	manualRuns sync.WaitGroup
	// shutdownTimeout limits how long Stop waits for running jobs.
	shutdownTimeout time.Duration
}

// Errors returned by RunNow and SetPaused.
var (
	ErrJobNotFound  = errors.New("cron job not found")
	ErrJobBusy      = errors.New("cron job is already running")
	ErrShuttingDown = errors.New("cronado is shutting down")
)

// CronJobManagerOptions contains configuration options for CronJobManager
type CronJobManagerOptions struct {
	Scheduler Scheduler
	Executor  CommandExecutor
	Metrics   MetricsCollector
	Notifier  NotificationService
	// ShutdownTimeout limits how long Stop waits for running jobs. Zero
	// waits until all of them finished.
	ShutdownTimeout time.Duration
}

// NewCronJobManager creates a new CronJobManager with dependency injection
func NewCronJobManager(opts CronJobManagerOptions) *CronJobManager {
	// Use default implementations if not provided
	if opts.Scheduler == nil {
		opts.Scheduler = &CronSchedulerAdapter{cron.New(cron.WithParser(cronParser))}
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
		labelIssues: make(map[string][]ContainerLabelIssue),
		scheduler:   opts.Scheduler,
		executor:    opts.Executor,
		metrics:     opts.Metrics,
		notifier:    opts.Notifier,
		stopping:    make(chan struct{}),
		jitterDelay: randomJitter,

		shutdownTimeout: opts.ShutdownTimeout,
	}
}

// randomJitter returns a random delay in [0, maxJitter).
func randomJitter(maxJitter time.Duration) time.Duration {
	return rand.N(maxJitter) //nolint:gosec // spreading load, not security relevant
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
	entryID, err := m.scheduler.AddFunc(job.SchedulerSpec(), func() {
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
		"schedule", job.SchedulerSpec(),
		"command", truncateForLog(job.Command, 80),
		"user", displayUser(job.User))
	slog.Debug("Cron job command detail", "job_id", jobID, "command", job.Command)

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
	if currentJob.Paused {
		slog.Info("Skipping paused cron job", "job_id", job.ID, "container", container.DisplayName(), "cron_job", job.Name)
		m.mu.Unlock()
		return
	}
	if m.isStandby(currentJob) {
		slog.Debug("Skipping cron job, another replica of the Compose service runs it", "job_id", job.ID, "container", container.DisplayName(), "cron_job", job.Name)
		m.mu.Unlock()
		return
	}
	if !currentJob.IsIdle() {
		slog.Warn("Skipping cron job execution - already running", "job_id", job.ID, "status", currentJob.Status)
		m.mu.Unlock()
		return
	}
	if job.Jitter > 0 {
		currentJob.SetPending()
		m.mu.Unlock()

		waited := m.waitJitter(job)

		m.mu.Lock()
		// The job may have been removed or re-registered during the wait.
		if m.jobs[job.ID] != currentJob {
			m.mu.Unlock()
			return
		}
		if !waited {
			currentJob.SetIdle()
			m.mu.Unlock()
			return
		}
	}
	currentJob.SetRunning()
	m.mu.Unlock()

	m.runCommand(container, job)
}

// RunNow starts a job immediately, without jitter and also while it is
// paused. The command runs in the background; RunNow returns ErrJobBusy if
// the job is already running or waiting out its jitter.
func (m *CronJobManager) RunNow(jobID string) error {
	m.mu.Lock()
	if m.isStopping() {
		m.mu.Unlock()
		return ErrShuttingDown
	}
	job, exists := m.jobs[jobID]
	if !exists {
		m.mu.Unlock()
		return ErrJobNotFound
	}
	if !job.IsIdle() {
		m.mu.Unlock()
		return ErrJobBusy
	}
	job.SetRunning()
	container := job.Container
	m.manualRuns.Add(1)
	m.mu.Unlock()

	slog.Info("Cron job triggered manually", "job_id", jobID, "container", container.DisplayName(), "cron_job", job.Name)
	go func() {
		defer m.manualRuns.Done()
		m.runCommand(container, job)
	}()
	return nil
}

// SetPaused pauses or resumes the scheduled runs of a job.
func (m *CronJobManager) SetPaused(jobID string, paused bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return ErrJobNotFound
	}
	if job.Paused != paused {
		job.Paused = paused
		action := "Cron job resumed"
		if paused {
			action = "Cron job paused"
		}
		slog.Info(action, "job_id", jobID, "container", job.GetContainerDisplayName(), "cron_job", job.Name)
	}
	return nil
}

// runCommand executes a job that has already been marked as running and
// records the result.
func (m *CronJobManager) runCommand(container *Container, job *CronJob) {
	startTime := time.Now()
	slog.Info("Executing cron job",
		"job_id", job.ID,
		"container", container.DisplayName(),
		"cron_job", job.Name)

	sendPing(job, pingStart, "")
	stdout, stderr, err := m.executor.ExecuteCommand(container, job.Name, job.Command, job.User, job.Timeout)
	duration := time.Since(startTime).Seconds()

	if stdout != "" {
		slog.Debug("Job stdout", "job_id", job.ID, "output", stdout)
	}
	if stderr != "" {
		slog.Debug("Job stderr", "job_id", job.ID, "error", stderr)
	}

	// Record metrics
	status := "success"
	if err != nil {
		status = "failure"
		slog.Error("Cron job execution failed",
			"job_id", job.ID,
			"container", container.DisplayName(),
			"cron_job", job.Name,
			"error", err)

		output := failureOutput(stdout, stderr)
		sendPing(job, pingFail, fmt.Sprintf("%v\n\n%s", err, output))

		// Send notification on failure
		if m.notifier != nil {
			if notifyErr := m.notifier.SendJobFailure(container, job.Name, err, output); notifyErr != nil {
				slog.Warn("Failed to send job failure notification", "error", notifyErr)
			}
		}
	} else {
		sendPing(job, pingSuccess, "")
	}

	m.metrics.RecordJobExecution(container.ShortID(), job.Name, status)
	m.metrics.RecordJobDuration(container.ShortID(), job.Name, duration)

	// Store the result and mark as idle after execution
	m.mu.Lock()
	if currentJob, exists := m.jobs[job.ID]; exists {
		currentJob.LastRun = &startTime
		currentJob.LastResult = status
		currentJob.LastDuration = duration
		currentJob.LastError = ""
		if err != nil {
			currentJob.LastError = err.Error()
		} else {
			currentJob.LastSuccess = &startTime
		}
		currentJob.SetIdle()
	}
	m.mu.Unlock()
}

// waitJitter sleeps for a random part of the job's jitter. It returns false
// if the manager is stopped during the wait.
func (m *CronJobManager) waitJitter(job *CronJob) bool {
	delay := m.jitterDelay(job.Jitter)
	if delay <= 0 {
		return true
	}
	slog.Debug("Delaying cron job by jitter", "job_id", job.ID, "delay", delay)

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-m.stopping:
		return false
	}
}

// failureOutput returns the output that best explains a failure: stderr if
// the command wrote any, stdout otherwise.
func failureOutput(stdout, stderr string) string {
	if strings.TrimSpace(stderr) != "" {
		return stderr
	}
	return stdout
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
		m.metrics.RemoveJobMetrics(job.GetShortContainerID(), job.Name)
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

	delete(m.labelIssues, containerID)

	for jobID, job := range m.jobs {
		if job.Container == nil || job.Container.ID != containerID {
			continue
		}
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
		m.metrics.RemoveJobMetrics(job.Container.ShortID(), job.Name)
	}
}

// GetAll retrieves all cron jobs (interface{} version for context compatibility)
func (m *CronJobManager) GetAll() []interface{} {
	jobs := m.GetAllTyped()
	result := make([]interface{}, 0, len(jobs))
	for _, job := range jobs {
		result = append(result, job)
	}
	return result
}

// GetAllTyped retrieves copies of all cron jobs, including their next run time
func (m *CronJobManager) GetAllTyped() []CronJob {
	// Read the schedule before taking the lock; the scheduler has its own.
	nextRuns := make(map[cron.EntryID]time.Time)
	for _, entry := range m.scheduler.Entries() {
		if !entry.Next.IsZero() {
			nextRuns[entry.ID] = entry.Next
		}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]CronJob, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobCopy := *job
		jobCopy.Standby = m.isStandby(job)
		// A paused or standby job has no next run, even though the scheduler
		// keeps it.
		if next, ok := nextRuns[job.SchedulerId]; ok && !job.Paused && !jobCopy.Standby {
			jobCopy.NextRun = &next
		}
		result = append(result, jobCopy)
	}
	return result
}

// isStandby reports whether the scheduled runs of job are left to another
// replica of the same Compose service: with replicas=once, only the job of
// the lowest-numbered replica runs, and the next one takes over when that
// container stops. The caller must hold m.mu.
func (m *CronJobManager) isStandby(job *CronJob) bool {
	if job.Replicas == ReplicasAll || job.Container == nil {
		return false
	}
	project, service, ok := job.Container.ComposeService()
	if !ok {
		return false
	}
	for _, other := range m.jobs {
		if other == job || other.Name != job.Name || other.Replicas == ReplicasAll || other.Container == nil {
			continue
		}
		otherProject, otherService, ok := other.Container.ComposeService()
		if ok && otherProject == project && otherService == service && other.Container.ranksBefore(job.Container) {
			return true
		}
	}
	return false
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

// Stop stops scheduling new runs and waits for running jobs. Runs waiting
// out their jitter are cancelled. With a shutdown timeout, Stop gives up
// waiting after it and logs the jobs that were still running; their commands
// keep running inside the containers, but their results are not recorded.
func (m *CronJobManager) Stop() {
	m.mu.Lock()
	m.stopOnce.Do(func() { close(m.stopping) })
	m.mu.Unlock()

	if running := m.runningJobs(); len(running) > 0 {
		slog.Info("Waiting for running cron jobs to finish", "jobs", len(running), "timeout", m.shutdownTimeout)
	}

	done := make(chan struct{})
	go func() {
		if ctx := m.scheduler.Stop(); ctx != nil {
			<-ctx.Done()
		}
		m.manualRuns.Wait()
		close(done)
	}()

	if m.shutdownTimeout <= 0 {
		<-done
		return
	}
	timer := time.NewTimer(m.shutdownTimeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		for _, job := range m.runningJobs() {
			slog.Warn("Shutdown timeout reached, abandoning running cron job; its command keeps running in the container but the result is not recorded",
				"job_id", job.ID,
				"container", job.GetContainerDisplayName(),
				"cron_job", job.Name,
				"timeout", m.shutdownTimeout)
		}
	}
}

// runningJobs returns copies of the jobs whose command is running.
func (m *CronJobManager) runningJobs() []CronJob {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []CronJob
	for _, job := range m.jobs {
		if job.IsRunning() {
			result = append(result, *job)
		}
	}
	return result
}

// isStopping reports whether Stop was called. The caller must hold m.mu.
func (m *CronJobManager) isStopping() bool {
	select {
	case <-m.stopping:
		return true
	default:
		return false
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

// displayUser describes the user a job runs as, for log output.
func displayUser(user string) string {
	if user == "" {
		return "(container default)"
	}
	return user
}

// truncateForLog shortens a string for log output, appending "..." if truncated.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
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

package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

// Mock implementations for testing

// MockScheduler implements the Scheduler interface for testing
type MockScheduler struct {
	mu            sync.Mutex
	entries       map[cron.EntryID]func()
	nextID        cron.EntryID
	started       bool
	addFuncCalled int
	removeCalled  int
}

func NewMockScheduler() *MockScheduler {
	return &MockScheduler{
		entries: make(map[cron.EntryID]func()),
		nextID:  1,
	}
}

func (m *MockScheduler) AddFunc(spec string, cmd func()) (cron.EntryID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.addFuncCalled++

	// Simulate invalid schedule error
	if spec == "invalid-schedule" {
		return 0, errors.New("invalid schedule")
	}

	id := m.nextID
	m.nextID++
	m.entries[id] = cmd
	return id, nil
}

func (m *MockScheduler) Remove(id cron.EntryID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.removeCalled++
	delete(m.entries, id)
}

func (m *MockScheduler) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = true
}

func (m *MockScheduler) Stop() context.Context {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = false

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel to simulate stop completion
	return ctx
}

func (m *MockScheduler) Entries() []cron.Entry {
	return []cron.Entry{} // Not needed for our tests
}

// Helper methods for testing
func (m *MockScheduler) GetAddFuncCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.addFuncCalled
}

func (m *MockScheduler) GetRemoveCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.removeCalled
}

func (m *MockScheduler) ExecuteJob(id cron.EntryID) {
	m.mu.Lock()
	cmd := m.entries[id]
	m.mu.Unlock()

	if cmd != nil {
		cmd()
	}
}

func (m *MockScheduler) IsStarted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.started
}

// MockCommandExecutor implements the CommandExecutor interface for testing
type MockCommandExecutor struct {
	mu             sync.Mutex
	executeCalled  int
	lastContainer  *Container
	lastJobName    string
	lastCommand    string
	lastUser       string
	shouldFail     bool
	executionDelay time.Duration
	stdout         string
	stderr         string
}

func NewMockCommandExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{}
}

func (m *MockCommandExecutor) ExecuteCommand(container *Container, jobName, command, user string, timeout time.Duration) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.executeCalled++
	m.lastContainer = container
	m.lastJobName = jobName
	m.lastCommand = command
	m.lastUser = user

	if m.executionDelay > 0 {
		time.Sleep(m.executionDelay)
	}

	if m.shouldFail {
		return "", "", errors.New("mock execution failed")
	}

	return m.stdout, m.stderr, nil
}

// Helper methods for testing
func (m *MockCommandExecutor) GetExecuteCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.executeCalled
}

func (m *MockCommandExecutor) GetLastExecution() (container *Container, jobName, command, user string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastContainer, m.lastJobName, m.lastCommand, m.lastUser
}

func (m *MockCommandExecutor) SetShouldFail(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFail = fail
}

func (m *MockCommandExecutor) SetExecutionDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executionDelay = delay
}

func (m *MockCommandExecutor) SetOutput(stdout, stderr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stdout = stdout
	m.stderr = stderr
}

// MockMetricsCollector implements the MetricsCollector interface for testing
type MockMetricsCollector struct {
	mu                    sync.Mutex
	incrementCalled       int
	decrementCalled       int
	recordExecutionCalled int
	recordDurationCalled  int
	lastContainerID       string
	lastJobName           string
	lastStatus            string
	lastDuration          float64
}

func NewMockMetricsCollector() *MockMetricsCollector {
	return &MockMetricsCollector{}
}

func (m *MockMetricsCollector) IncrementScheduledJobs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.incrementCalled++
}

func (m *MockMetricsCollector) DecrementScheduledJobs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.decrementCalled++
}

func (m *MockMetricsCollector) RecordJobExecution(containerID, jobName, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordExecutionCalled++
	m.lastContainerID = containerID
	m.lastJobName = jobName
	m.lastStatus = status
}

func (m *MockMetricsCollector) RecordJobDuration(containerID, jobName string, duration float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordDurationCalled++
	m.lastContainerID = containerID
	m.lastJobName = jobName
	m.lastDuration = duration
}

// Helper methods for testing
func (m *MockMetricsCollector) GetIncrementCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.incrementCalled
}

func (m *MockMetricsCollector) GetDecrementCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.decrementCalled
}

func (m *MockMetricsCollector) GetRecordExecutionCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.recordExecutionCalled
}

func (m *MockMetricsCollector) GetRecordDurationCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.recordDurationCalled
}

func (m *MockMetricsCollector) GetLastExecution() (containerID, jobName, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastContainerID, m.lastJobName, m.lastStatus
}

func (m *MockMetricsCollector) GetLastDuration() (containerID, jobName string, duration float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastContainerID, m.lastJobName, m.lastDuration
}

// MockNotificationService implements the NotificationService interface for testing
type MockNotificationService struct {
	mu                     sync.Mutex
	sendNotificationCalled int
	sendJobFailureCalled   int
	lastTitle              string
	lastMessage            string
	lastContainer          *Container
	lastJobName            string
	lastError              error
	shouldFail             bool
}

func NewMockNotificationService() *MockNotificationService {
	return &MockNotificationService{}
}

func (m *MockNotificationService) SendNotification(title, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sendNotificationCalled++
	m.lastTitle = title
	m.lastMessage = message

	if m.shouldFail {
		return errors.New("mock notification failed")
	}

	return nil
}

func (m *MockNotificationService) SendJobFailure(container *Container, jobName string, err error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sendJobFailureCalled++
	m.lastContainer = container
	m.lastJobName = jobName
	m.lastError = err

	if m.shouldFail {
		return errors.New("mock notification failed")
	}

	return nil
}

// Helper methods for testing
func (m *MockNotificationService) GetSendNotificationCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sendNotificationCalled
}

func (m *MockNotificationService) GetSendJobFailureCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sendJobFailureCalled
}

func (m *MockNotificationService) GetLastNotification() (title, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastTitle, m.lastMessage
}

func (m *MockNotificationService) GetLastJobFailure() (container *Container, jobName string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastContainer, m.lastJobName, m.lastError
}

func (m *MockNotificationService) SetShouldFail(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFail = fail
}

// Test helper functions
func createTestCronJobManagerContainer() *Container {
	return &Container{
		ID:   "abcdef123456789",
		Name: "test-container",
		Labels: map[string]string{
			"app": "test",
		},
	}
}

func createTestCronJob(container *Container) CronJob {
	return CronJob{
		ID:        "test-123-backup",
		Name:      "backup",
		Container: container,
		Enabled:   true,
		Schedule:  "@every 1h",
		Command:   "echo backup",
		User:      "root",
		Status:    StatusIdle,
	}
}

func createTestCronJobManager() (*CronJobManager, *MockScheduler, *MockCommandExecutor, *MockMetricsCollector, *MockNotificationService) {
	scheduler := NewMockScheduler()
	executor := NewMockCommandExecutor()
	metrics := NewMockMetricsCollector()
	notifier := NewMockNotificationService()

	manager := NewCronJobManager(CronJobManagerOptions{
		Scheduler: scheduler,
		Executor:  executor,
		Metrics:   metrics,
		Notifier:  notifier,
	})

	return manager, scheduler, executor, metrics, notifier
}

// Actual tests

func TestNewCronJobManager(t *testing.T) {
	t.Run("with all dependencies provided", func(t *testing.T) {
		manager, scheduler, executor, metrics, notifier := createTestCronJobManager()

		if manager == nil {
			t.Fatal("Expected non-nil manager")
		}

		if !manager.initialized {
			t.Error("Expected manager to be initialized")
		}

		if manager.GetJobCount() != 0 {
			t.Errorf("Expected job count to be 0, got %d", manager.GetJobCount())
		}

		// Verify dependencies are set
		if scheduler == nil || executor == nil || metrics == nil || notifier == nil {
			t.Error("Expected all dependencies to be set")
		}
	})

	t.Run("with default dependencies", func(t *testing.T) {
		manager := NewCronJobManager(CronJobManagerOptions{})

		if manager == nil {
			t.Fatal("Expected non-nil manager")
		}

		if !manager.initialized {
			t.Error("Expected manager to be initialized")
		}
	})
}

func TestCronJobManager_Add(t *testing.T) {
	t.Run("successful job addition", func(t *testing.T) {
		manager, scheduler, _, metrics, _ := createTestCronJobManager()
		container := createTestCronJobManagerContainer()
		job := createTestCronJob(container)

		err := manager.Add(container, job)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify job was added
		if manager.GetJobCount() != 1 {
			t.Errorf("Expected job count to be 1, got %d", manager.GetJobCount())
		}

		// Verify scheduler was called
		if scheduler.GetAddFuncCallCount() != 1 {
			t.Errorf("Expected scheduler AddFunc to be called once, got %d", scheduler.GetAddFuncCallCount())
		}

		// Verify metrics were incremented
		if metrics.GetIncrementCallCount() != 1 {
			t.Errorf("Expected metrics increment to be called once, got %d", metrics.GetIncrementCallCount())
		}

		// Verify job is registered
		if !manager.IsRegistered(job.ID) {
			t.Error("Expected job to be registered")
		}
	})

	t.Run("invalid schedule error", func(t *testing.T) {
		manager, scheduler, _, metrics, _ := createTestCronJobManager()
		container := createTestCronJobManagerContainer()
		job := createTestCronJob(container)
		job.Schedule = "invalid-schedule" // This will cause mock to return error

		err := manager.Add(container, job)
		if err == nil {
			t.Fatal("Expected error for invalid schedule")
		}

		// Verify job was not added
		if manager.GetJobCount() != 0 {
			t.Errorf("Expected job count to be 0, got %d", manager.GetJobCount())
		}

		// Verify scheduler was called but failed
		if scheduler.GetAddFuncCallCount() != 1 {
			t.Errorf("Expected scheduler AddFunc to be called once, got %d", scheduler.GetAddFuncCallCount())
		}

		// Verify metrics were not incremented on error
		if metrics.GetIncrementCallCount() != 0 {
			t.Errorf("Expected metrics increment not to be called on error, got %d", metrics.GetIncrementCallCount())
		}
	})

	t.Run("concurrent job additions", func(t *testing.T) {
		manager, _, _, metrics, _ := createTestCronJobManager()
		container := createTestCronJobManagerContainer()

		const numJobs = 10
		var wg sync.WaitGroup
		errors := make(chan error, numJobs)

		for i := 0; i < numJobs; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				job := createTestCronJob(container)
				job.ID = fmt.Sprintf("test-123-job%d", i)
				job.Name = fmt.Sprintf("job%d", i)
				err := manager.Add(container, job)
				errors <- err
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			if err != nil {
				t.Errorf("Unexpected error in concurrent addition: %v", err)
			}
		}

		// Verify all jobs were added
		if manager.GetJobCount() != numJobs {
			t.Errorf("Expected job count to be %d, got %d", numJobs, manager.GetJobCount())
		}

		// Verify metrics were incremented correctly
		if metrics.GetIncrementCallCount() != numJobs {
			t.Errorf("Expected metrics increment to be called %d times, got %d", numJobs, metrics.GetIncrementCallCount())
		}
	})
}

func TestCronJobManager_ExecuteJob(t *testing.T) {
	t.Run("successful job execution", func(t *testing.T) {
		manager, scheduler, executor, metrics, _ := createTestCronJobManager()
		container := createTestCronJobManagerContainer()
		job := createTestCronJob(container)

		// Add job
		err := manager.Add(container, job)
		if err != nil {
			t.Fatalf("Failed to add job: %v", err)
		}

		// Get the job to find its scheduler ID
		retrievedJob, found := manager.GetJobByID(job.ID)
		if !found {
			t.Fatal("Job not found after adding")
		}

		// Execute the job via scheduler
		scheduler.ExecuteJob(retrievedJob.SchedulerId)

		// Wait a bit for async execution
		time.Sleep(10 * time.Millisecond)

		// Verify executor was called
		if executor.GetExecuteCallCount() != 1 {
			t.Errorf("Expected executor to be called once, got %d", executor.GetExecuteCallCount())
		}

		// Verify execution parameters
		execContainer, execJobName, execCommand, execUser := executor.GetLastExecution()
		if execContainer.ID != container.ID {
			t.Errorf("Expected container ID %q, got %q", container.ID, execContainer.ID)
		}
		if execJobName != job.Name {
			t.Errorf("Expected job name %q, got %q", job.Name, execJobName)
		}
		if execCommand != job.Command {
			t.Errorf("Expected command %q, got %q", job.Command, execCommand)
		}
		if execUser != job.User {
			t.Errorf("Expected user %q, got %q", job.User, execUser)
		}

		// Verify metrics were recorded
		if metrics.GetRecordExecutionCallCount() != 1 {
			t.Errorf("Expected metrics RecordExecution to be called once, got %d", metrics.GetRecordExecutionCallCount())
		}

		if metrics.GetRecordDurationCallCount() != 1 {
			t.Errorf("Expected metrics RecordDuration to be called once, got %d", metrics.GetRecordDurationCallCount())
		}

		// Verify success metrics
		containerID, jobName, status := metrics.GetLastExecution()
		if containerID != container.ShortID() {
			t.Errorf("Expected container ID %q, got %q", container.ShortID(), containerID)
		}
		if jobName != job.Name {
			t.Errorf("Expected job name %q, got %q", job.Name, jobName)
		}
		if status != "success" {
			t.Errorf("Expected status %q, got %q", "success", status)
		}
	})

	t.Run("job execution failure", func(t *testing.T) {
		manager, scheduler, executor, metrics, notifier := createTestCronJobManager()
		container := createTestCronJobManagerContainer()
		job := createTestCronJob(container)

		// Set executor to fail
		executor.SetShouldFail(true)

		// Add job
		err := manager.Add(container, job)
		if err != nil {
			t.Fatalf("Failed to add job: %v", err)
		}

		// Get the job to find its scheduler ID
		retrievedJob, found := manager.GetJobByID(job.ID)
		if !found {
			t.Fatal("Job not found after adding")
		}

		// Execute the job via scheduler
		scheduler.ExecuteJob(retrievedJob.SchedulerId)

		// Wait a bit for async execution
		time.Sleep(10 * time.Millisecond)

		// Verify failure metrics
		_, _, status := metrics.GetLastExecution()
		if status != "failure" {
			t.Errorf("Expected status %q, got %q", "failure", status)
		}

		// Verify notification was sent
		if notifier.GetSendJobFailureCallCount() != 1 {
			t.Errorf("Expected SendJobFailure to be called once, got %d", notifier.GetSendJobFailureCallCount())
		}

		// Verify notification parameters
		notifContainer, notifJobName, notifErr := notifier.GetLastJobFailure()
		if notifContainer.ID != container.ID {
			t.Errorf("Expected notification container ID %q, got %q", container.ID, notifContainer.ID)
		}
		if notifJobName != job.Name {
			t.Errorf("Expected notification job name %q, got %q", job.Name, notifJobName)
		}
		if notifErr == nil {
			t.Error("Expected notification error to be non-nil")
		}
	})

	t.Run("concurrent execution prevention", func(t *testing.T) {
		manager, scheduler, executor, _, _ := createTestCronJobManager()
		container := createTestCronJobManagerContainer()
		job := createTestCronJob(container)

		// Set executor to have delay to simulate long-running job
		executor.SetExecutionDelay(50 * time.Millisecond)

		// Add job
		err := manager.Add(container, job)
		if err != nil {
			t.Fatalf("Failed to add job: %v", err)
		}

		// Get the job to find its scheduler ID
		retrievedJob, found := manager.GetJobByID(job.ID)
		if !found {
			t.Fatal("Job not found after adding")
		}

		// Execute the job twice concurrently
		go scheduler.ExecuteJob(retrievedJob.SchedulerId)
		go scheduler.ExecuteJob(retrievedJob.SchedulerId)

		// Wait for executions to complete
		time.Sleep(100 * time.Millisecond)

		// Should only execute once due to running status check
		if executor.GetExecuteCallCount() != 1 {
			t.Errorf("Expected executor to be called once (concurrent execution prevented), got %d", executor.GetExecuteCallCount())
		}
	})
}

// Continuing with remaining test methods...
func TestCronJobManager_Remove(t *testing.T) {
	t.Run("remove jobs by container ID", func(t *testing.T) {
		manager, scheduler, _, metrics, _ := createTestCronJobManager()
		container := createTestCronJobManagerContainer()

		// Add multiple jobs for the same container
		job1 := createTestCronJob(container)
		job1.ID = "test-123-job1"
		job1.Name = "job1"

		job2 := createTestCronJob(container)
		job2.ID = "test-123-job2"
		job2.Name = "job2"

		manager.Add(container, job1)
		manager.Add(container, job2)

		if manager.GetJobCount() != 2 {
			t.Fatalf("Expected 2 jobs, got %d", manager.GetJobCount())
		}

		// Remove all jobs for container
		manager.Remove(container.ID)

		// Verify jobs were removed
		if manager.GetJobCount() != 0 {
			t.Errorf("Expected 0 jobs after removal, got %d", manager.GetJobCount())
		}

		// Verify scheduler Remove was called twice
		if scheduler.GetRemoveCallCount() != 2 {
			t.Errorf("Expected scheduler Remove to be called twice, got %d", scheduler.GetRemoveCallCount())
		}

		// Verify metrics were decremented
		if metrics.GetDecrementCallCount() != 2 {
			t.Errorf("Expected metrics decrement to be called twice, got %d", metrics.GetDecrementCallCount())
		}

		// Verify jobs are not registered
		if manager.IsRegistered(job1.ID) {
			t.Error("Expected job1 to not be registered")
		}
		if manager.IsRegistered(job2.ID) {
			t.Error("Expected job2 to not be registered")
		}
	})

	t.Run("remove non-existent container", func(t *testing.T) {
		manager, scheduler, _, metrics, _ := createTestCronJobManager()

		// Remove jobs for non-existent container
		manager.Remove("non-existent")

		// Should not cause errors and should not call scheduler/metrics
		if scheduler.GetRemoveCallCount() != 0 {
			t.Errorf("Expected scheduler Remove not to be called, got %d", scheduler.GetRemoveCallCount())
		}

		if metrics.GetDecrementCallCount() != 0 {
			t.Errorf("Expected metrics decrement not to be called, got %d", metrics.GetDecrementCallCount())
		}
	})
}

func TestCronJobManager_RemoveJob(t *testing.T) {
	manager, scheduler, _, metrics, _ := createTestCronJobManager()
	container := createTestCronJobManagerContainer()
	job := createTestCronJob(container)

	// Add job
	manager.Add(container, job)

	// Get the actual job with scheduler ID
	retrievedJob, found := manager.GetJobByID(job.ID)
	if !found {
		t.Fatal("Job not found after adding")
	}

	// Remove specific job
	manager.RemoveJob(*retrievedJob)

	// Verify job was removed
	if manager.GetJobCount() != 0 {
		t.Errorf("Expected 0 jobs after removal, got %d", manager.GetJobCount())
	}

	// Verify scheduler Remove was called
	if scheduler.GetRemoveCallCount() != 1 {
		t.Errorf("Expected scheduler Remove to be called once, got %d", scheduler.GetRemoveCallCount())
	}

	// Verify metrics were decremented
	if metrics.GetDecrementCallCount() != 1 {
		t.Errorf("Expected metrics decrement to be called once, got %d", metrics.GetDecrementCallCount())
	}

	// Verify job is not registered
	if manager.IsRegistered(job.ID) {
		t.Error("Expected job to not be registered")
	}
}

func TestCronJobManager_GetAll(t *testing.T) {
	manager, _, _, _, _ := createTestCronJobManager()
	container := createTestCronJobManagerContainer()

	// Initially empty
	jobs := manager.GetAll()
	if len(jobs) != 0 {
		t.Errorf("Expected 0 jobs initially, got %d", len(jobs))
	}

	// Add some jobs
	job1 := createTestCronJob(container)
	job1.ID = "test-123-job1"
	job1.Name = "job1"

	job2 := createTestCronJob(container)
	job2.ID = "test-123-job2"
	job2.Name = "job2"

	manager.Add(container, job1)
	manager.Add(container, job2)

	// Get all jobs
	jobs = manager.GetAll()
	if len(jobs) != 2 {
		t.Errorf("Expected 2 jobs, got %d", len(jobs))
	}

	// Verify job data (order may vary)
	jobNames := make(map[string]bool)
	for _, jobInterface := range jobs {
		job, ok := jobInterface.(CronJob)
		if !ok {
			t.Error("Expected job to be of type CronJob")
			continue
		}
		jobNames[job.Name] = true
	}

	if !jobNames["job1"] {
		t.Error("Expected to find job1")
	}
	if !jobNames["job2"] {
		t.Error("Expected to find job2")
	}
}

func TestCronJobManager_IsRegistered(t *testing.T) {
	manager, _, _, _, _ := createTestCronJobManager()
	container := createTestCronJobManagerContainer()
	job := createTestCronJob(container)

	// Initially not registered
	if manager.IsRegistered(job.ID) {
		t.Error("Expected job not to be registered initially")
	}

	// Add job
	manager.Add(container, job)

	// Should be registered
	if !manager.IsRegistered(job.ID) {
		t.Error("Expected job to be registered after adding")
	}

	// Remove job
	retrievedJob, _ := manager.GetJobByID(job.ID)
	manager.RemoveJob(*retrievedJob)

	// Should not be registered
	if manager.IsRegistered(job.ID) {
		t.Error("Expected job not to be registered after removal")
	}
}

func TestCronJobManager_GetJobByID(t *testing.T) {
	manager, _, _, _, _ := createTestCronJobManager()
	container := createTestCronJobManagerContainer()
	job := createTestCronJob(container)

	// Non-existent job
	_, found := manager.GetJobByID(job.ID)
	if found {
		t.Error("Expected job not to be found initially")
	}

	// Add job
	manager.Add(container, job)

	// Should find job
	retrievedJob, found := manager.GetJobByID(job.ID)
	if !found {
		t.Error("Expected job to be found after adding")
	}

	if retrievedJob.ID != job.ID {
		t.Errorf("Expected job ID %q, got %q", job.ID, retrievedJob.ID)
	}

	if retrievedJob.Name != job.Name {
		t.Errorf("Expected job name %q, got %q", job.Name, retrievedJob.Name)
	}

	// Verify it's a copy (modifying shouldn't affect original)
	retrievedJob.Name = "modified"

	retrievedJob2, _ := manager.GetJobByID(job.ID)
	if retrievedJob2.Name == "modified" {
		t.Error("Expected GetJobByID to return a copy, not the original")
	}
}

func TestCronJobManager_StartStop(t *testing.T) {
	manager, scheduler, _, _, _ := createTestCronJobManager()

	// Start
	manager.Start()
	if !scheduler.IsStarted() {
		t.Error("Expected scheduler to be started")
	}

	// Stop
	manager.Stop()
	if scheduler.IsStarted() {
		t.Error("Expected scheduler to be stopped")
	}

	// Shutdown (alias for Stop)
	manager.Start() // Start again
	manager.Shutdown()
	if scheduler.IsStarted() {
		t.Error("Expected scheduler to be stopped after shutdown")
	}
}

func TestCronJobManager_GetJobCount(t *testing.T) {
	manager, _, _, _, _ := createTestCronJobManager()
	container := createTestCronJobManagerContainer()

	// Initially 0
	if manager.GetJobCount() != 0 {
		t.Errorf("Expected job count to be 0 initially, got %d", manager.GetJobCount())
	}

	// Add jobs
	job1 := createTestCronJob(container)
	job1.ID = "test-123-job1"

	job2 := createTestCronJob(container)
	job2.ID = "test-123-job2"

	manager.Add(container, job1)
	if manager.GetJobCount() != 1 {
		t.Errorf("Expected job count to be 1 after adding one job, got %d", manager.GetJobCount())
	}

	manager.Add(container, job2)
	if manager.GetJobCount() != 2 {
		t.Errorf("Expected job count to be 2 after adding two jobs, got %d", manager.GetJobCount())
	}

	// Remove jobs
	manager.Remove(container.ID)
	if manager.GetJobCount() != 0 {
		t.Errorf("Expected job count to be 0 after removing all jobs, got %d", manager.GetJobCount())
	}
}

func TestTruncateForLog(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 80, "short"},
		{"", 80, ""},
		{"exactly80chars" + strings.Repeat("x", 66), 80, "exactly80chars" + strings.Repeat("x", 66)},
		{"exactly81chars" + strings.Repeat("x", 67), 80, "exactly81chars" + strings.Repeat("x", 66) + "..."},
		{strings.Repeat("a", 200), 80, strings.Repeat("a", 80) + "..."},
	}
	for _, tt := range tests {
		got := truncateForLog(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateForLog(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

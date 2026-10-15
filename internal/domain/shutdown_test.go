package domain

import (
	"errors"
	"testing"
	"time"
)

func newShutdownManager(timeout time.Duration) (*CronJobManager, *MockCommandExecutor) {
	executor := NewMockCommandExecutor()
	manager := NewCronJobManager(CronJobManagerOptions{
		Scheduler:       NewMockScheduler(),
		Executor:        executor,
		Metrics:         NewMockMetricsCollector(),
		Notifier:        NewMockNotificationService(),
		ShutdownTimeout: timeout,
	})
	return manager, executor
}

func TestStop_GivesUpAfterShutdownTimeout(t *testing.T) {
	manager, executor := newShutdownManager(50 * time.Millisecond)
	executor.SetExecutionDelay(2 * time.Second)
	job := addTestJob(t, manager, 0)
	// RunNow marks the job as running before it returns.
	if err := manager.RunNow(job.ID); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	manager.Stop()
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Stop took %s, want about the 50ms shutdown timeout", elapsed)
	}
	if stored, _ := manager.GetJobByID(job.ID); !stored.IsRunning() {
		t.Errorf("status = %q, want the abandoned job still running", stored.Status)
	}
}

func TestStop_ReturnsWhenJobsFinishBeforeTimeout(t *testing.T) {
	manager, executor := newShutdownManager(5 * time.Second)
	executor.SetExecutionDelay(50 * time.Millisecond)
	job := addTestJob(t, manager, 0)
	if err := manager.RunNow(job.ID); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	manager.Stop()
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Stop took %s, want it to return once the job finished", elapsed)
	}
	if stored, _ := manager.GetJobByID(job.ID); stored.LastResult != "success" {
		t.Errorf("LastResult = %q, want success", stored.LastResult)
	}
}

func TestRunNow_RefusedDuringShutdown(t *testing.T) {
	manager, executor := newShutdownManager(time.Second)
	job := addTestJob(t, manager, 0)

	manager.Stop()
	if err := manager.RunNow(job.ID); !errors.Is(err, ErrShuttingDown) {
		t.Errorf("err = %v, want ErrShuttingDown", err)
	}
	if executor.GetExecuteCallCount() != 0 {
		t.Errorf("executions = %d, want 0", executor.GetExecuteCallCount())
	}
}

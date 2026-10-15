package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func addTestJob(t *testing.T, manager *CronJobManager, jitter time.Duration) CronJob {
	t.Helper()
	container := createTestContainer("test-123")
	job := CronJob{
		ID:        "test-123-job",
		Name:      "job",
		Container: container,
		Enabled:   true,
		Schedule:  "@every 1h",
		Command:   "echo hello",
		Jitter:    jitter,
		Status:    StatusIdle,
	}
	if err := manager.Add(container, job); err != nil {
		t.Fatalf("Add: %v", err)
	}
	stored, _ := manager.GetJobByID(job.ID)
	return *stored
}

func waitForExecutions(t *testing.T, executor *MockCommandExecutor, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if executor.GetExecuteCallCount() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("executions = %d, want %d", executor.GetExecuteCallCount(), want)
}

func TestRunNow_RunsImmediatelyWithoutJitterAndWhilePaused(t *testing.T) {
	manager, _, executor, _, _ := createTestCronJobManager()
	manager.jitterDelay = func(time.Duration) time.Duration { return time.Hour }
	job := addTestJob(t, manager, 2*time.Hour)
	if err := manager.SetPaused(job.ID, true); err != nil {
		t.Fatal(err)
	}

	if err := manager.RunNow(job.ID); err != nil {
		t.Fatalf("RunNow: %v", err)
	}
	waitForExecutions(t, executor, 1)
	waitForStatus(t, manager, job.ID, StatusIdle)

	stored, _ := manager.GetJobByID(job.ID)
	if stored.LastResult != "success" || !stored.Paused {
		t.Errorf("LastResult = %q, Paused = %v; want success and still paused", stored.LastResult, stored.Paused)
	}
}

func TestRunNow_Errors(t *testing.T) {
	manager, _, executor, _, _ := createTestCronJobManager()
	executor.SetExecutionDelay(200 * time.Millisecond)
	job := addTestJob(t, manager, 0)

	if err := manager.RunNow("missing"); !errors.Is(err, ErrJobNotFound) {
		t.Errorf("unknown job: err = %v, want ErrJobNotFound", err)
	}
	if err := manager.RunNow(job.ID); err != nil {
		t.Fatalf("first RunNow: %v", err)
	}
	if err := manager.RunNow(job.ID); !errors.Is(err, ErrJobBusy) {
		t.Errorf("second RunNow while running: err = %v, want ErrJobBusy", err)
	}
	manager.Stop() // waits for the manual run
	if executor.GetExecuteCallCount() != 1 {
		t.Errorf("executions = %d, want 1", executor.GetExecuteCallCount())
	}
}

func TestStop_WaitsForManualRuns(t *testing.T) {
	manager, _, executor, _, _ := createTestCronJobManager()
	executor.SetExecutionDelay(100 * time.Millisecond)
	job := addTestJob(t, manager, 0)

	if err := manager.RunNow(job.ID); err != nil {
		t.Fatal(err)
	}
	manager.Stop()

	if stored, _ := manager.GetJobByID(job.ID); stored.LastResult != "success" {
		t.Errorf("Stop returned before the manual run finished: LastResult = %q", stored.LastResult)
	}
}

func TestSetPaused_SkipsScheduledRunsUntilResumed(t *testing.T) {
	manager, scheduler, executor, _, _ := createTestCronJobManager()
	scheduler.SetNext(time.Now().Add(time.Hour))
	job := addTestJob(t, manager, 0)

	if err := manager.SetPaused(job.ID, true); err != nil {
		t.Fatal(err)
	}
	scheduler.ExecuteJob(job.SchedulerId)
	if executor.GetExecuteCallCount() != 0 {
		t.Errorf("paused job ran %d times", executor.GetExecuteCallCount())
	}
	if all := manager.GetAllTyped(); len(all) != 1 || all[0].NextRun != nil || !all[0].Paused {
		t.Errorf("paused job = %+v, want Paused and no NextRun", all)
	}
	expected := `
# HELP cronado_job_paused 1 if the cron job is paused, 0 otherwise
# TYPE cronado_job_paused gauge
cronado_job_paused{container_id="test-123",container_name="test-container",job_name="job"} 1
`
	if err := testutil.CollectAndCompare(NewJobStateCollector(manager), strings.NewReader(expected), "cronado_job_paused"); err != nil {
		t.Error(err)
	}

	if err := manager.SetPaused(job.ID, false); err != nil {
		t.Fatal(err)
	}
	scheduler.ExecuteJob(job.SchedulerId)
	if executor.GetExecuteCallCount() != 1 {
		t.Errorf("resumed job ran %d times, want 1", executor.GetExecuteCallCount())
	}
	if all := manager.GetAllTyped(); all[0].NextRun == nil {
		t.Error("resumed job has no NextRun")
	}

	if err := manager.SetPaused("missing", true); !errors.Is(err, ErrJobNotFound) {
		t.Errorf("unknown job: err = %v, want ErrJobNotFound", err)
	}
}

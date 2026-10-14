package domain

import (
	"strings"
	"testing"
	"time"
)

func TestCronJobBuilder_SetJitter(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      time.Duration
		wantError bool
	}{
		{name: "minutes", input: "5m", want: 5 * time.Minute},
		{name: "with spaces", input: " 30s ", want: 30 * time.Second},
		{name: "zero disables", input: "0", want: 0},
		{name: "empty keeps default", input: "", want: 0},
		{name: "negative", input: "-1m", wantError: true},
		{name: "not a duration", input: "five minutes", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewCronJobBuilder("test-job", createTestContainer("test-123"), 0, 0)
			builder.SetJitter(tt.input)

			if builder.HasErrors() != tt.wantError {
				t.Fatalf("HasErrors() = %v, want %v (errors: %v)", builder.HasErrors(), tt.wantError, builder.GetErrors())
			}
			if builder.jitter != tt.want {
				t.Errorf("jitter = %s, want %s", builder.jitter, tt.want)
			}
		})
	}
}

func TestCronJobBuilder_JitterMustBeShorterThanInterval(t *testing.T) {
	tests := []struct {
		schedule  string
		jitter    string
		wantError bool
	}{
		{schedule: "0 * * * *", jitter: "5m"},
		{schedule: "@every 10m", jitter: "9m59s"},
		{schedule: "@every 10m", jitter: "10m", wantError: true},
		{schedule: "*/5 * * * *", jitter: "1h", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.schedule+" jitter "+tt.jitter, func(t *testing.T) {
			builder := NewCronJobBuilder("test-job", createTestContainer("test-123"), 0, 0)
			builder.SetEnabled("true")
			builder.SetJitter(tt.jitter) // labels arrive in any order
			builder.SetSchedule(tt.schedule)
			builder.SetCommand("echo hello")

			job, err := builder.Build()
			if tt.wantError {
				if err == nil || !strings.Contains(err.Error(), "shorter than the schedule interval") {
					t.Fatalf("expected interval error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if want, _ := time.ParseDuration(tt.jitter); job.Jitter != want {
				t.Errorf("job.Jitter = %s, want %s", job.Jitter, want)
			}
		})
	}
}

// addJitterJob registers a job with jitter and returns it with the scheduler
// entry that triggers it.
func addJitterJob(t *testing.T, manager *CronJobManager, jitter time.Duration) CronJob {
	t.Helper()
	container := createTestContainer("test-123")
	job := CronJob{
		ID:        "test-123-jittered",
		Name:      "jittered",
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

func TestCronJobManager_JitterDelaysExecution(t *testing.T) {
	manager, scheduler, executor, _, _ := createTestCronJobManager()
	var gotMax time.Duration
	manager.jitterDelay = func(maxJitter time.Duration) time.Duration {
		gotMax = maxJitter
		return 50 * time.Millisecond
	}
	job := addJitterJob(t, manager, 5*time.Minute)

	start := time.Now()
	scheduler.ExecuteJob(job.SchedulerId)

	if gotMax != 5*time.Minute {
		t.Errorf("jitterDelay called with %s, want 5m", gotMax)
	}
	if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
		t.Errorf("run started after %s, want at least 50ms", elapsed)
	}
	if executor.GetExecuteCallCount() != 1 {
		t.Errorf("executions = %d, want 1", executor.GetExecuteCallCount())
	}
}

func TestCronJobManager_JitterSkipsOverlappingTrigger(t *testing.T) {
	manager, scheduler, executor, _, _ := createTestCronJobManager()
	manager.jitterDelay = func(time.Duration) time.Duration { return 100 * time.Millisecond }
	job := addJitterJob(t, manager, time.Minute)

	done := make(chan struct{})
	go func() {
		scheduler.ExecuteJob(job.SchedulerId)
		close(done)
	}()
	waitForStatus(t, manager, job.ID, StatusPending)

	scheduler.ExecuteJob(job.SchedulerId) // second trigger while pending
	<-done

	if executor.GetExecuteCallCount() != 1 {
		t.Errorf("executions = %d, want 1", executor.GetExecuteCallCount())
	}
}

func TestCronJobManager_StopAbortsJitterWait(t *testing.T) {
	manager, scheduler, executor, _, _ := createTestCronJobManager()
	manager.jitterDelay = func(time.Duration) time.Duration { return time.Hour }
	job := addJitterJob(t, manager, 2*time.Hour)

	done := make(chan struct{})
	go func() {
		scheduler.ExecuteJob(job.SchedulerId)
		close(done)
	}()
	waitForStatus(t, manager, job.ID, StatusPending)

	manager.Stop()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("run still waiting after Stop")
	}

	if executor.GetExecuteCallCount() != 0 {
		t.Errorf("executions = %d, want 0", executor.GetExecuteCallCount())
	}
	if stored, _ := manager.GetJobByID(job.ID); !stored.IsIdle() {
		t.Errorf("status = %q, want idle", stored.Status)
	}
}

func TestCronJobManager_RemovedDuringJitterIsNotExecuted(t *testing.T) {
	manager, scheduler, executor, _, _ := createTestCronJobManager()
	manager.jitterDelay = func(time.Duration) time.Duration { return 100 * time.Millisecond }
	job := addJitterJob(t, manager, time.Minute)
	entry := job.SchedulerId

	done := make(chan struct{})
	go func() {
		scheduler.ExecuteJob(entry)
		close(done)
	}()
	waitForStatus(t, manager, job.ID, StatusPending)

	manager.Remove(job.Container.ID)
	<-done

	if executor.GetExecuteCallCount() != 0 {
		t.Errorf("executions = %d, want 0", executor.GetExecuteCallCount())
	}
}

func waitForStatus(t *testing.T, manager *CronJobManager, jobID, status string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if job, ok := manager.GetJobByID(jobID); ok && job.Status == status {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %s never reached status %q", jobID, status)
}

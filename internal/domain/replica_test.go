package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func composeReplica(id, service, number string) *Container {
	return &Container{
		ID:   id,
		Name: "app-" + service + "-" + number,
		Labels: map[string]string{
			composeProjectLabel: "app",
			composeServiceLabel: service,
			composeNumberLabel:  number,
			composeOneoffLabel:  "False",
		},
		Status: "running",
	}
}

func addReplicaJob(t *testing.T, manager *CronJobManager, container *Container, replicas string) CronJob {
	t.Helper()
	job := CronJob{
		ID:        container.ShortID() + "-cleanup",
		Name:      "cleanup",
		Container: container,
		Enabled:   true,
		Schedule:  "@every 1h",
		Command:   "echo cleanup",
		Replicas:  replicas,
		Status:    StatusIdle,
	}
	if err := manager.Add(container, job); err != nil {
		t.Fatalf("Add: %v", err)
	}
	stored, _ := manager.GetJobByID(job.ID)
	return *stored
}

func standbyByID(manager *CronJobManager) map[string]bool {
	result := make(map[string]bool)
	for _, job := range manager.GetAllTyped() {
		result[job.ID] = job.Standby
	}
	return result
}

func TestReplicas_OnlyLowestReplicaRunsAndNextTakesOver(t *testing.T) {
	manager, scheduler, executor, _, _ := createTestCronJobManager()
	scheduler.SetNext(time.Now().Add(time.Hour))
	// Registered in reverse order, as event order is not guaranteed.
	second := addReplicaJob(t, manager, composeReplica("bbbbbbbbbbbb2", "web", "2"), ReplicasOnce)
	first := addReplicaJob(t, manager, composeReplica("aaaaaaaaaaaa1", "web", "1"), ReplicasOnce)

	scheduler.ExecuteJob(second.SchedulerId)
	if n := executor.GetExecuteCallCount(); n != 0 {
		t.Fatalf("standby replica ran %d times", n)
	}
	scheduler.ExecuteJob(first.SchedulerId)
	if n := executor.GetExecuteCallCount(); n != 1 {
		t.Fatalf("first replica ran %d times, want 1", n)
	}

	standby := standbyByID(manager)
	if standby[first.ID] || !standby[second.ID] {
		t.Errorf("standby = %v, want only %s", standby, second.ID)
	}
	for _, job := range manager.GetAllTyped() {
		if job.ID == second.ID && job.NextRun != nil {
			t.Errorf("standby job has next_run %v", job.NextRun)
		}
	}

	expected := `
# HELP cronado_job_standby 1 if another replica of the Compose service runs this cron job, 0 otherwise
# TYPE cronado_job_standby gauge
cronado_job_standby{container_id="aaaaaaaaaaaa",container_name="app-web-1",job_name="cleanup"} 0
cronado_job_standby{container_id="bbbbbbbbbbbb",container_name="app-web-2",job_name="cleanup"} 1
`
	if err := testutil.CollectAndCompare(NewJobStateCollector(manager), strings.NewReader(expected), "cronado_job_standby"); err != nil {
		t.Error(err)
	}

	manager.Remove(first.Container.ID)
	scheduler.ExecuteJob(second.SchedulerId)
	if n := executor.GetExecuteCallCount(); n != 2 {
		t.Errorf("after the first replica stopped, executions = %d, want 2", n)
	}
	if standbyByID(manager)[second.ID] {
		t.Error("remaining replica is still standby")
	}
}

func TestReplicas_AllRunsOnEveryReplica(t *testing.T) {
	manager, scheduler, executor, _, _ := createTestCronJobManager()
	first := addReplicaJob(t, manager, composeReplica("aaaaaaaaaaaa1", "web", "1"), ReplicasAll)
	second := addReplicaJob(t, manager, composeReplica("bbbbbbbbbbbb2", "web", "2"), ReplicasAll)

	scheduler.ExecuteJob(first.SchedulerId)
	scheduler.ExecuteJob(second.SchedulerId)
	if n := executor.GetExecuteCallCount(); n != 2 {
		t.Errorf("executions = %d, want 2", n)
	}
	if standby := standbyByID(manager); standby[first.ID] || standby[second.ID] {
		t.Errorf("standby = %v, want none", standby)
	}
}

func TestReplicas_OtherServicesAndNonComposeContainersRun(t *testing.T) {
	manager, scheduler, executor, _, _ := createTestCronJobManager()
	web := addReplicaJob(t, manager, composeReplica("aaaaaaaaaaaa1", "web", "1"), ReplicasOnce)
	worker := addReplicaJob(t, manager, composeReplica("bbbbbbbbbbbb2", "worker", "2"), ReplicasOnce)
	plain := addReplicaJob(t, manager, &Container{ID: "cccccccccccc3", Name: "plain", Status: "running"}, ReplicasOnce)

	for _, job := range []CronJob{web, worker, plain} {
		scheduler.ExecuteJob(job.SchedulerId)
	}
	if n := executor.GetExecuteCallCount(); n != 3 {
		t.Errorf("executions = %d, want 3", n)
	}
}

func TestReplicas_OneoffContainerIsStandby(t *testing.T) {
	manager, scheduler, executor, _, _ := createTestCronJobManager()
	replica := composeReplica("bbbbbbbbbbbb2", "web", "2")
	oneoff := composeReplica("aaaaaaaaaaaa1", "web", "1")
	oneoff.Labels[composeOneoffLabel] = "True"
	oneoffJob := addReplicaJob(t, manager, oneoff, ReplicasOnce)
	addReplicaJob(t, manager, replica, ReplicasOnce)

	scheduler.ExecuteJob(oneoffJob.SchedulerId)
	if n := executor.GetExecuteCallCount(); n != 0 {
		t.Errorf("one-off container ran the job %d times while a replica is running", n)
	}
}

func TestReplicas_RunNowRunsOnStandbyReplica(t *testing.T) {
	manager, _, executor, _, _ := createTestCronJobManager()
	addReplicaJob(t, manager, composeReplica("aaaaaaaaaaaa1", "web", "1"), ReplicasOnce)
	second := addReplicaJob(t, manager, composeReplica("bbbbbbbbbbbb2", "web", "2"), ReplicasOnce)

	if err := manager.RunNow(second.ID); err != nil {
		t.Fatalf("RunNow: %v", err)
	}
	waitForExecutions(t, executor, 1)
	waitForStatus(t, manager, second.ID, StatusIdle)
}

func TestReplicasLabel(t *testing.T) {
	tests := []struct {
		value   string
		want    string
		wantErr bool
	}{
		{"", ReplicasOnce, false},
		{"once", ReplicasOnce, false},
		{" ALL ", ReplicasAll, false},
		{"some", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			labels := map[string]string{
				"cronado.job.enabled":  "true",
				"cronado.job.schedule": "@every 1h",
				"cronado.job.cmd":      "echo hi",
			}
			if tt.value != "" {
				labels["cronado.job.replicas"] = tt.value
			}
			jobs, issues := ParseJobLabels(&Container{ID: "abcdefabcdef", Labels: labels}, "cronado", 0, 0)
			if tt.wantErr {
				if len(issues) != 1 || !issues[0].JobRejected || !strings.Contains(issues[0].Message, "invalid replicas value") {
					t.Errorf("issues = %+v, want one invalid replicas issue", issues)
				}
				return
			}
			if len(issues) != 0 {
				t.Fatalf("issues = %+v", issues)
			}
			if got := jobs["job"].Replicas; got != tt.want {
				t.Errorf("Replicas = %q, want %q", got, tt.want)
			}
		})
	}
}

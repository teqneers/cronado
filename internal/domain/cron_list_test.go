package domain

import (
	"testing"

	"github.com/teqneers/cronado/internal/context"
)

func TestGetCronJobs_Empty(t *testing.T) {
	manager, _, _, _, _ := createTestCronJobManager()
	context.AppCtx = &context.AppContext{
		CronJobManager: manager,
	}

	result := GetCronJobs()
	if len(result) != 0 {
		t.Errorf("expected 0 items, got %d", len(result))
	}
}

func TestGetCronJobs_WithJobs(t *testing.T) {
	manager, _, _, _, _ := createTestCronJobManager()
	container := &Container{
		ID:   "abcdef123456789",
		Name: "my-container",
	}
	context.AppCtx = &context.AppContext{
		CronJobManager: manager,
	}

	job1 := CronJob{
		ID:       "abc-job1",
		Name:     "job1",
		Container: container,
		Enabled:  true,
		Schedule: "@every 1h",
		Command:  "echo hello",
		User:     "root",
		Status:   StatusIdle,
	}
	job2 := CronJob{
		ID:       "abc-job2",
		Name:     "job2",
		Container: container,
		Enabled:  true,
		Schedule: "@every 5m",
		Command:  "date",
		User:     "www-data",
		Status:   StatusIdle,
	}

	manager.Add(container, job1)
	manager.Add(container, job2)

	result := GetCronJobs()
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}

	// Verify container info is populated
	for _, item := range result {
		if item.ContainerID != container.ID {
			t.Errorf("expected container ID %q, got %q", container.ID, item.ContainerID)
		}
		if item.ContainerName != container.Name {
			t.Errorf("expected container name %q, got %q", container.Name, item.ContainerName)
		}
	}
}

func TestGetCronJobs_NilContainer(t *testing.T) {
	manager, _, _, _, _ := createTestCronJobManager()
	container := &Container{
		ID:   "abcdef123456789",
		Name: "",
	}
	context.AppCtx = &context.AppContext{
		CronJobManager: manager,
	}

	job := CronJob{
		ID:       "abc-job1",
		Name:     "job1",
		Container: container,
		Enabled:  true,
		Schedule: "@every 1h",
		Command:  "echo hello",
		User:     "root",
		Status:   StatusIdle,
	}

	manager.Add(container, job)

	result := GetCronJobs()
	if len(result) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result))
	}

	if result[0].ContainerName != "" {
		t.Errorf("expected empty container name, got %q", result[0].ContainerName)
	}
}

func TestCronListItem_Structure(t *testing.T) {
	item := CronListItem{
		ContainerID:   "abc123",
		ContainerName: "my-container",
		CronJob: CronJob{
			ID:   "abc-backup",
			Name: "backup",
		},
	}

	if item.ContainerID != "abc123" {
		t.Errorf("ContainerID = %q, want %q", item.ContainerID, "abc123")
	}
	if item.ContainerName != "my-container" {
		t.Errorf("ContainerName = %q, want %q", item.ContainerName, "my-container")
	}
	if item.CronJob.Name != "backup" {
		t.Errorf("CronJob.Name = %q, want %q", item.CronJob.Name, "backup")
	}
}

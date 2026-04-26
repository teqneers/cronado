package domain

import (
	"testing"

	"github.com/teqneers/cronado/internal/config"
	cronadoCtx "github.com/teqneers/cronado/internal/context"
)

// Integration test for cron parsing from container labels
func TestParseCronsFromContainerIntegration(t *testing.T) {
	tests := []struct {
		name           string
		labels         map[string]string
		cronPrefix     string
		expectedJobs   int
		expectedJobIDs []string
	}{
		{
			name: "single enabled cron job",
			labels: map[string]string{
				"cronado.backup.enabled":  "true",
				"cronado.backup.schedule": "@daily",
				"cronado.backup.cmd":      "echo backup",
				"cronado.backup.user":     "root",
			},
			cronPrefix:     "cronado",
			expectedJobs:   1,
			expectedJobIDs: []string{"test-123-backup"},
		},
		{
			name: "multiple cron jobs",
			labels: map[string]string{
				"cronado.backup.enabled":   "true",
				"cronado.backup.schedule":  "@daily",
				"cronado.backup.cmd":       "echo backup",
				"cronado.cleanup.enabled":  "true",
				"cronado.cleanup.schedule": "@hourly",
				"cronado.cleanup.cmd":      "rm -rf /tmp/*",
			},
			cronPrefix:     "cronado",
			expectedJobs:   2,
			expectedJobIDs: []string{"test-123-backup", "test-123-cleanup"},
		},
		{
			name: "disabled cron job",
			labels: map[string]string{
				"cronado.backup.enabled":  "false",
				"cronado.backup.schedule": "@daily",
				"cronado.backup.cmd":      "echo backup",
			},
			cronPrefix:     "cronado",
			expectedJobs:   1, // parseCronsFromContainer returns all jobs, filtering happens in handleContainer
			expectedJobIDs: []string{"test-123-backup"},
		},
		{
			name: "missing required fields",
			labels: map[string]string{
				"cronado.backup.enabled": "true",
				"cronado.backup.cmd":     "echo backup",
				// missing schedule
			},
			cronPrefix:   "cronado",
			expectedJobs: 0,
		},
		{
			name: "invalid label format",
			labels: map[string]string{
				"cronado.backup":          "invalid",
				"cronado.backup.enabled":  "true",
				"cronado.backup.schedule": "@daily",
				"cronado.backup.cmd":      "echo backup",
			},
			cronPrefix:     "cronado",
			expectedJobs:   1, // Should still parse the valid ones
			expectedJobIDs: []string{"test-123-backup"},
		},
		{
			name: "no cron labels",
			labels: map[string]string{
				"app":     "my-app",
				"version": "1.0.0",
			},
			cronPrefix:   "cronado",
			expectedJobs: 0,
		},
		{
			name: "custom prefix",
			labels: map[string]string{
				"custom.job.enabled":  "true",
				"custom.job.schedule": "@every 5m",
				"custom.job.cmd":      "echo custom",
			},
			cronPrefix:     "custom",
			expectedJobs:   1,
			expectedJobIDs: []string{"test-123-job"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up global context for parseCronsFromContainer
			cronadoCtx.AppCtx = &cronadoCtx.AppContext{
				Config: &config.Config{
					CronLabelPrefix: tt.cronPrefix,
				},
			}

			container := &Container{
				ID:     "test-123",
				Name:   "test-container",
				Labels: tt.labels,
			}

			cronJobs := parseCronsFromContainer(container, tt.cronPrefix)

			if len(cronJobs) != tt.expectedJobs {
				t.Errorf("Expected %d cron jobs, got %d", tt.expectedJobs, len(cronJobs))
			}

			// Verify specific job IDs if provided
			if len(tt.expectedJobIDs) > 0 {
				for _, expectedID := range tt.expectedJobIDs {
					found := false
					for _, job := range cronJobs {
						if job.ID == expectedID {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected job ID %s not found in parsed jobs", expectedID)
					}
				}
			}

			// Verify all parsed jobs are valid
			for _, job := range cronJobs {
				if !job.IsValid() {
					t.Errorf("Parsed job %s is not valid", job.ID)
				}
			}
		})
	}
}

// Integration test for handleContainer function with mocked manager
func TestHandleContainerIntegration(t *testing.T) {
	// Save original context
	originalContext := cronadoCtx.AppCtx
	defer func() { cronadoCtx.AppCtx = originalContext }()

	t.Run("handle container with multiple jobs", func(t *testing.T) {
		// Create test manager with mocks for this specific test
		mockScheduler := NewMockScheduler()
		mockExecutor := &MockCommandExecutor{}
		mockMetrics := &MockMetricsCollector{}
		mockNotifier := &MockNotificationService{}

		manager := NewCronJobManager(CronJobManagerOptions{
			Scheduler: mockScheduler,
			Executor:  mockExecutor,
			Metrics:   mockMetrics,
			Notifier:  mockNotifier,
		})

		// Set up global context for testing
		testContext := &cronadoCtx.AppContext{
			CronJobManager: manager,
			Config: &config.Config{
				CronLabelPrefix: "cronado",
			},
		}
		cronadoCtx.AppCtx = testContext
		container := &Container{
			ID:   "multi-job-container",
			Name: "multi-container",
			Labels: map[string]string{
				"cronado.backup.enabled":    "true",
				"cronado.backup.schedule":   "@daily",
				"cronado.backup.cmd":        "backup.sh",
				"cronado.cleanup.enabled":   "true",
				"cronado.cleanup.schedule":  "@hourly",
				"cronado.cleanup.cmd":       "cleanup.sh",
				"cronado.disabled.enabled":  "false",
				"cronado.disabled.schedule": "@weekly",
				"cronado.disabled.cmd":      "disabled.sh",
			},
		}

		handleContainer(container)

		// Should register 2 jobs (backup and cleanup), skip the disabled one
		if manager.GetJobCount() != 2 {
			t.Errorf("Expected 2 jobs to be registered, got %d", manager.GetJobCount())
		}

		if !manager.IsRegistered("multi-job-co-backup") {
			t.Error("Expected backup job to be registered")
		}

		if !manager.IsRegistered("multi-job-co-cleanup") {
			t.Error("Expected cleanup job to be registered")
		}

		if manager.IsRegistered("multi-job-co-disabled") {
			t.Error("Did not expect disabled job to be registered")
		}

		// Verify mock scheduler was called
		if mockScheduler.addFuncCalled != 2 {
			t.Errorf("Expected scheduler AddFunc to be called 2 times, got %d", mockScheduler.addFuncCalled)
		}
	})

	t.Run("handle container with no cron jobs", func(t *testing.T) {
		// Create fresh test manager with mocks for this specific test
		mockScheduler := NewMockScheduler()
		mockExecutor := &MockCommandExecutor{}
		mockMetrics := &MockMetricsCollector{}
		mockNotifier := &MockNotificationService{}

		manager := NewCronJobManager(CronJobManagerOptions{
			Scheduler: mockScheduler,
			Executor:  mockExecutor,
			Metrics:   mockMetrics,
			Notifier:  mockNotifier,
		})

		// Set up global context for testing
		testContext := &cronadoCtx.AppContext{
			CronJobManager: manager,
			Config: &config.Config{
				CronLabelPrefix: "cronado",
			},
		}
		cronadoCtx.AppCtx = testContext

		container := &Container{
			ID:   "no-cron-container",
			Name: "no-cron",
			Labels: map[string]string{
				"app":     "test-app",
				"version": "1.0.0",
			},
		}

		handleContainer(container)

		// Should not register any jobs
		if manager.GetJobCount() != 0 {
			t.Errorf("Expected 0 jobs for container without cron labels, got %d", manager.GetJobCount())
		}
	})

	t.Run("handle container with already registered job", func(t *testing.T) {
		// Create fresh test manager with mocks for this specific test
		mockScheduler := NewMockScheduler()
		mockExecutor := &MockCommandExecutor{}
		mockMetrics := &MockMetricsCollector{}
		mockNotifier := &MockNotificationService{}

		manager := NewCronJobManager(CronJobManagerOptions{
			Scheduler: mockScheduler,
			Executor:  mockExecutor,
			Metrics:   mockMetrics,
			Notifier:  mockNotifier,
		})

		// Set up global context for testing
		testContext := &cronadoCtx.AppContext{
			CronJobManager: manager,
			Config: &config.Config{
				CronLabelPrefix: "cronado",
			},
		}
		cronadoCtx.AppCtx = testContext

		container := &Container{
			ID:   "existing-job-container",
			Name: "existing-container",
			Labels: map[string]string{
				"cronado.existing.enabled":  "true",
				"cronado.existing.schedule": "@daily",
				"cronado.existing.cmd":      "existing.sh",
			},
		}

		// First call should register the job
		handleContainer(container)
		if manager.GetJobCount() != 1 {
			t.Errorf("Expected 1 job after first handleContainer call, got %d", manager.GetJobCount())
		}

		initialAddFuncCalls := mockScheduler.addFuncCalled

		// Second call with same container should skip (already registered)
		handleContainer(container)

		// Job count should remain the same
		if manager.GetJobCount() != 1 {
			t.Errorf("Expected 1 job after second handleContainer call, got %d", manager.GetJobCount())
		}

		// AddFunc should not be called again for the same job
		if mockScheduler.addFuncCalled != initialAddFuncCalls {
			t.Errorf("Expected AddFunc not to be called again, but got %d calls (was %d)",
				mockScheduler.addFuncCalled, initialAddFuncCalls)
		}
	})
}

// Integration test for registerCronJob function
func TestRegisterCronJobIntegration(t *testing.T) {
	originalContext := cronadoCtx.AppCtx
	defer func() { cronadoCtx.AppCtx = originalContext }()

	t.Run("register valid cron job", func(t *testing.T) {
		// Create fresh test manager with mocks for this specific test
		mockScheduler := NewMockScheduler()
		mockExecutor := &MockCommandExecutor{}
		mockMetrics := &MockMetricsCollector{}
		mockNotifier := &MockNotificationService{}

		manager := NewCronJobManager(CronJobManagerOptions{
			Scheduler: mockScheduler,
			Executor:  mockExecutor,
			Metrics:   mockMetrics,
			Notifier:  mockNotifier,
		})

		testContext := &cronadoCtx.AppContext{
			CronJobManager: manager,
			Config: &config.Config{
				CronLabelPrefix: "cronado",
			},
		}
		cronadoCtx.AppCtx = testContext
		container := &Container{
			ID:   "valid-job-container",
			Name: "valid-container",
		}

		cronJob := CronJob{
			ID:        "valid-job-container-valid",
			Name:      "valid",
			Schedule:  "@daily",
			Command:   "echo valid",
			Enabled:   true,
			Container: container,
		}

		registerCronJob(container, cronJob)

		if !manager.IsRegistered(cronJob.ID) {
			t.Error("Expected valid job to be registered")
		}
	})

	t.Run("skip invalid cron job", func(t *testing.T) {
		// Create fresh test manager with mocks for this specific test
		mockScheduler := NewMockScheduler()
		mockExecutor := &MockCommandExecutor{}
		mockMetrics := &MockMetricsCollector{}
		mockNotifier := &MockNotificationService{}

		manager := NewCronJobManager(CronJobManagerOptions{
			Scheduler: mockScheduler,
			Executor:  mockExecutor,
			Metrics:   mockMetrics,
			Notifier:  mockNotifier,
		})

		testContext := &cronadoCtx.AppContext{
			CronJobManager: manager,
			Config: &config.Config{
				CronLabelPrefix: "cronado",
			},
		}
		cronadoCtx.AppCtx = testContext
		container := &Container{
			ID:   "invalid-job-container",
			Name: "invalid-container",
		}

		// Invalid job - missing required fields
		invalidJob := CronJob{
			ID:   "invalid-job-container-invalid",
			Name: "invalid",
			// Missing Schedule and Command
			Enabled: true,
		}

		initialJobCount := manager.GetJobCount()
		registerCronJob(container, invalidJob)

		// Job count should not change
		if manager.GetJobCount() != initialJobCount {
			t.Errorf("Expected job count to remain %d for invalid job, got %d",
				initialJobCount, manager.GetJobCount())
		}

		if manager.IsRegistered(invalidJob.ID) {
			t.Error("Did not expect invalid job to be registered")
		}
	})

	t.Run("skip already registered job", func(t *testing.T) {
		// Create fresh test manager with mocks for this specific test
		mockScheduler := NewMockScheduler()
		mockExecutor := &MockCommandExecutor{}
		mockMetrics := &MockMetricsCollector{}
		mockNotifier := &MockNotificationService{}

		manager := NewCronJobManager(CronJobManagerOptions{
			Scheduler: mockScheduler,
			Executor:  mockExecutor,
			Metrics:   mockMetrics,
			Notifier:  mockNotifier,
		})

		testContext := &cronadoCtx.AppContext{
			CronJobManager: manager,
			Config: &config.Config{
				CronLabelPrefix: "cronado",
			},
		}
		cronadoCtx.AppCtx = testContext
		container := &Container{
			ID:   "duplicate-job-container",
			Name: "duplicate-container",
		}

		cronJob := CronJob{
			ID:        "duplicate-job-container-duplicate",
			Name:      "duplicate",
			Schedule:  "@hourly",
			Command:   "echo duplicate",
			Enabled:   true,
			Container: container,
		}

		// Register once
		registerCronJob(container, cronJob)
		initialJobCount := manager.GetJobCount()
		initialAddFuncCalls := mockScheduler.addFuncCalled

		// Try to register again
		registerCronJob(container, cronJob)

		// Job count should not change
		if manager.GetJobCount() != initialJobCount {
			t.Errorf("Expected job count to remain %d for duplicate job, got %d",
				initialJobCount, manager.GetJobCount())
		}

		// AddFunc should not be called again
		if mockScheduler.addFuncCalled != initialAddFuncCalls {
			t.Errorf("Expected AddFunc not to be called again for duplicate, but got %d calls (was %d)",
				mockScheduler.addFuncCalled, initialAddFuncCalls)
		}
	})
}

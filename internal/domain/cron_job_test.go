package domain

import (
	"testing"

	"github.com/robfig/cron/v3"
)

func TestCronJob_GetShortContainerID(t *testing.T) {
	tests := []struct {
		name     string
		cronJob  CronJob
		expected string
	}{
		{
			name: "with container",
			cronJob: CronJob{
				Container: &Container{ID: "abcdef123456789"},
			},
			expected: "abcdef123456",
		},
		{
			name: "without container",
			cronJob: CronJob{
				Container: nil,
			},
			expected: "unknown",
		},
		{
			name: "with empty container ID",
			cronJob: CronJob{
				Container: &Container{ID: ""},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cronJob.GetShortContainerID()
			if result != tt.expected {
				t.Errorf("GetShortContainerID() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestCronJob_GetContainerDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		cronJob  CronJob
		expected string
	}{
		{
			name: "with named container",
			cronJob: CronJob{
				Container: &Container{
					ID:   "abcdef123456789",
					Name: "my-container",
				},
			},
			expected: "my-container (abcdef123456)",
		},
		{
			name: "with container without name",
			cronJob: CronJob{
				Container: &Container{
					ID:   "abcdef123456789",
					Name: "",
				},
			},
			expected: "abcdef123456",
		},
		{
			name: "without container",
			cronJob: CronJob{
				Container: nil,
			},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cronJob.GetContainerDisplayName()
			if result != tt.expected {
				t.Errorf("GetContainerDisplayName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestCronJob_IsValid(t *testing.T) {
	validContainer := &Container{
		ID:   "abcdef123456789",
		Name: "test-container",
	}

	tests := []struct {
		name     string
		cronJob  CronJob
		expected bool
	}{
		{
			name: "valid cron job",
			cronJob: CronJob{
				Name:      "test-job",
				Container: validContainer,
				Schedule:  "@every 5m",
				Command:   "echo hello",
			},
			expected: true,
		},
		{
			name: "empty name",
			cronJob: CronJob{
				Name:      "",
				Container: validContainer,
				Schedule:  "@every 5m",
				Command:   "echo hello",
			},
			expected: false,
		},
		{
			name: "nil container",
			cronJob: CronJob{
				Name:      "test-job",
				Container: nil,
				Schedule:  "@every 5m",
				Command:   "echo hello",
			},
			expected: false,
		},
		{
			name: "empty container ID",
			cronJob: CronJob{
				Name: "test-job",
				Container: &Container{
					ID:   "",
					Name: "test-container",
				},
				Schedule: "@every 5m",
				Command:  "echo hello",
			},
			expected: false,
		},
		{
			name: "empty schedule",
			cronJob: CronJob{
				Name:      "test-job",
				Container: validContainer,
				Schedule:  "",
				Command:   "echo hello",
			},
			expected: false,
		},
		{
			name: "empty command",
			cronJob: CronJob{
				Name:      "test-job",
				Container: validContainer,
				Schedule:  "@every 5m",
				Command:   "",
			},
			expected: false,
		},
		{
			name: "all fields empty",
			cronJob: CronJob{
				Name:      "",
				Container: nil,
				Schedule:  "",
				Command:   "",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cronJob.IsValid()
			if result != tt.expected {
				t.Errorf("IsValid() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCronJob_StatusMethods(t *testing.T) {
	cronJob := CronJob{
		Status: StatusIdle,
	}

	// Test initial state (idle)
	if !cronJob.IsIdle() {
		t.Error("Expected cronJob to be idle initially")
	}
	if cronJob.IsRunning() {
		t.Error("Expected cronJob not to be running initially")
	}

	// Test setting to running
	cronJob.SetRunning()
	if cronJob.Status != StatusRunning {
		t.Errorf("Status = %q, want %q", cronJob.Status, StatusRunning)
	}
	if !cronJob.IsRunning() {
		t.Error("Expected cronJob to be running after SetRunning()")
	}
	if cronJob.IsIdle() {
		t.Error("Expected cronJob not to be idle after SetRunning()")
	}

	// Test setting back to idle
	cronJob.SetIdle()
	if cronJob.Status != StatusIdle {
		t.Errorf("Status = %q, want %q", cronJob.Status, StatusIdle)
	}
	if !cronJob.IsIdle() {
		t.Error("Expected cronJob to be idle after SetIdle()")
	}
	if cronJob.IsRunning() {
		t.Error("Expected cronJob not to be running after SetIdle()")
	}
}

func TestCronJob_StatusConstants(t *testing.T) {
	if StatusIdle != "idle" {
		t.Errorf("StatusIdle = %q, want %q", StatusIdle, "idle")
	}
	if StatusRunning != "running" {
		t.Errorf("StatusRunning = %q, want %q", StatusRunning, "running")
	}
}

func TestCronJob_FieldsAndSerialization(t *testing.T) {
	container := &Container{
		ID:   "abcdef123456789",
		Name: "test-container",
	}

	cronJob := CronJob{
		ID:          "test-id",
		Name:        "test-job",
		Container:   container,
		Enabled:     true,
		Schedule:    "@every 1h",
		Command:     "echo test",
		User:        "root",
		Status:      StatusIdle,
		SchedulerId: cron.EntryID(123),
	}

	// Test all fields are set correctly
	if cronJob.ID != "test-id" {
		t.Errorf("ID = %q, want %q", cronJob.ID, "test-id")
	}
	if cronJob.Name != "test-job" {
		t.Errorf("Name = %q, want %q", cronJob.Name, "test-job")
	}
	if cronJob.Container != container {
		t.Errorf("Container = %v, want %v", cronJob.Container, container)
	}
	if !cronJob.Enabled {
		t.Error("Expected Enabled to be true")
	}
	if cronJob.Schedule != "@every 1h" {
		t.Errorf("Schedule = %q, want %q", cronJob.Schedule, "@every 1h")
	}
	if cronJob.Command != "echo test" {
		t.Errorf("Command = %q, want %q", cronJob.Command, "echo test")
	}
	if cronJob.User != "root" {
		t.Errorf("User = %q, want %q", cronJob.User, "root")
	}
	if cronJob.Status != StatusIdle {
		t.Errorf("Status = %q, want %q", cronJob.Status, StatusIdle)
	}
	if cronJob.SchedulerId != cron.EntryID(123) {
		t.Errorf("SchedulerId = %v, want %v", cronJob.SchedulerId, cron.EntryID(123))
	}
}

func TestCronJob_EdgeCases(t *testing.T) {
	t.Run("status transitions with unknown status", func(t *testing.T) {
		cronJob := CronJob{
			Status: "unknown",
		}

		if cronJob.IsIdle() {
			t.Error("Expected cronJob with unknown status not to be idle")
		}
		if cronJob.IsRunning() {
			t.Error("Expected cronJob with unknown status not to be running")
		}

		// Should still be able to set status
		cronJob.SetRunning()
		if !cronJob.IsRunning() {
			t.Error("Expected cronJob to be running after SetRunning()")
		}
	})

	t.Run("validation with whitespace fields", func(t *testing.T) {
		container := &Container{
			ID:   "abcdef123456789",
			Name: "test-container",
		}

		cronJob := CronJob{
			Name:      "   ", // whitespace only
			Container: container,
			Schedule:  "@every 5m",
			Command:   "echo hello",
		}

		// Note: Current implementation doesn't trim whitespace in IsValid()
		// It treats whitespace-only as a valid non-empty string
		// This might be a candidate for improvement in the future
		if !cronJob.IsValid() {
			t.Error("Current implementation treats whitespace-only name as valid")
		}
	})

	t.Run("container methods with complex container names", func(t *testing.T) {
		container := &Container{
			ID:   "abcdef123456789",
			Name: "/container-with-slashes_and_underscores-123",
		}

		cronJob := CronJob{
			Container: container,
		}

		displayName := cronJob.GetContainerDisplayName()
		expected := "/container-with-slashes_and_underscores-123 (abcdef123456)"
		if displayName != expected {
			t.Errorf("GetContainerDisplayName() = %q, want %q", displayName, expected)
		}
	})
}

// Benchmark tests for performance-critical methods
func BenchmarkCronJob_IsValid(b *testing.B) {
	container := &Container{
		ID:   "abcdef123456789",
		Name: "test-container",
	}

	cronJob := CronJob{
		Name:      "test-job",
		Container: container,
		Schedule:  "@every 5m",
		Command:   "echo hello",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cronJob.IsValid()
	}
}

func BenchmarkCronJob_StatusCheck(b *testing.B) {
	cronJob := CronJob{
		Status: StatusRunning,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cronJob.IsRunning()
	}
}

func BenchmarkCronJob_GetShortContainerID(b *testing.B) {
	cronJob := CronJob{
		Container: &Container{ID: "abcdef123456789"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cronJob.GetShortContainerID()
	}
}

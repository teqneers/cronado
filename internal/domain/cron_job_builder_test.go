package domain

import (
	"fmt"
	"testing"
)

// Helper function to create a test container
func createTestContainer(id string) *Container {
	return &Container{
		ID:   id,
		Name: "test-container",
		Labels: map[string]string{
			"app": "test",
		},
	}
}

func TestCronJobBuilder_SetEnabled(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantEnabled *bool
		wantError   bool
	}{
		{
			name:        "true value",
			input:       "true",
			wantEnabled: boolPtr(true),
			wantError:   false,
		},
		{
			name:        "false value",
			input:       "false",
			wantEnabled: boolPtr(false),
			wantError:   false,
		},
		{
			name:        "1 as true",
			input:       "1",
			wantEnabled: boolPtr(true),
			wantError:   false,
		},
		{
			name:        "0 as false",
			input:       "0",
			wantEnabled: boolPtr(false),
			wantError:   false,
		},
		{
			name:        "yes as true",
			input:       "yes",
			wantEnabled: boolPtr(true),
			wantError:   false,
		},
		{
			name:        "no as false",
			input:       "no",
			wantEnabled: boolPtr(false),
			wantError:   false,
		},
		{
			name:        "on as true",
			input:       "on",
			wantEnabled: boolPtr(true),
			wantError:   false,
		},
		{
			name:        "off as false",
			input:       "off",
			wantEnabled: boolPtr(false),
			wantError:   false,
		},
		{
			name:        "empty string as false",
			input:       "",
			wantEnabled: boolPtr(false),
			wantError:   false,
		},
		{
			name:        "whitespace trimming",
			input:       "  true  ",
			wantEnabled: boolPtr(true),
			wantError:   false,
		},
		{
			name:        "case insensitive",
			input:       "TRUE",
			wantEnabled: boolPtr(true),
			wantError:   false,
		},
		{
			name:        "invalid value",
			input:       "invalid",
			wantEnabled: nil,
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := createTestContainer("test-123")
			builder := NewCronJobBuilder("test-job", container)
			builder.SetEnabled(tt.input)

			if tt.wantError {
				if !builder.HasErrors() {
					t.Errorf("expected error for input %q", tt.input)
				}
			} else {
				if builder.HasErrors() {
					t.Errorf("unexpected error for input %q: %v", tt.input, builder.GetErrors())
				}
				if !compareBoolPtr(builder.enabled, tt.wantEnabled) {
					t.Errorf("enabled = %v, want %v", builder.enabled, tt.wantEnabled)
				}
			}
		})
	}
}

func TestCronJobBuilder_SetSchedule(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantSchedule string
		wantError    bool
	}{
		{
			name:         "valid cron expression (5 fields)",
			input:        "0 * * * *",
			wantSchedule: "0 * * * *",
			wantError:    false,
		},
		{
			name:         "valid cron expression (6 fields)",
			input:        "0 0 * * * *",
			wantSchedule: "0 0 * * * *",
			wantError:    false,
		},
		{
			name:         "valid @every syntax",
			input:        "@every 5m",
			wantSchedule: "@every 5m",
			wantError:    false,
		},
		{
			name:         "valid @hourly syntax",
			input:        "@hourly",
			wantSchedule: "@hourly",
			wantError:    false,
		},
		{
			name:         "whitespace trimming",
			input:        "  0 * * * *  ",
			wantSchedule: "0 * * * *",
			wantError:    false,
		},
		{
			name:         "empty schedule",
			input:        "",
			wantSchedule: "",
			wantError:    true,
		},
		{
			name:         "invalid cron expression (too few fields)",
			input:        "0 * *",
			wantSchedule: "",
			wantError:    true,
		},
		{
			name:         "invalid cron expression (too many fields)",
			input:        "0 0 0 * * * *",
			wantSchedule: "",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := createTestContainer("test-123")
			builder := NewCronJobBuilder("test-job", container)
			builder.SetSchedule(tt.input)

			if tt.wantError {
				if !builder.HasErrors() {
					t.Errorf("expected error for schedule %q", tt.input)
				}
			} else {
				if builder.HasErrors() {
					t.Errorf("unexpected error for schedule %q: %v", tt.input, builder.GetErrors())
				}
				if builder.schedule != tt.wantSchedule {
					t.Errorf("schedule = %q, want %q", builder.schedule, tt.wantSchedule)
				}
			}
		})
	}
}

func TestCronJobBuilder_SetCommand(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantCommand string
		wantError   bool
	}{
		{
			name:        "valid command",
			input:       "echo hello",
			wantCommand: "echo hello",
			wantError:   false,
		},
		{
			name:        "whitespace trimming",
			input:       "  echo hello  ",
			wantCommand: "echo hello",
			wantError:   false,
		},
		{
			name:        "empty command",
			input:       "",
			wantCommand: "",
			wantError:   true,
		},
		{
			name:        "whitespace only",
			input:       "   ",
			wantCommand: "",
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := createTestContainer("test-123")
			builder := NewCronJobBuilder("test-job", container)
			builder.SetCommand(tt.input)

			if tt.wantError {
				if !builder.HasErrors() {
					t.Errorf("expected error for command %q", tt.input)
				}
			} else {
				if builder.HasErrors() {
					t.Errorf("unexpected error for command %q: %v", tt.input, builder.GetErrors())
				}
				if builder.command != tt.wantCommand {
					t.Errorf("command = %q, want %q", builder.command, tt.wantCommand)
				}
			}
		})
	}
}

func TestCronJobBuilder_SetUser(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantUser string
	}{
		{
			name:     "valid user",
			input:    "www-data",
			wantUser: "www-data",
		},
		{
			name:     "whitespace trimming",
			input:    "  www-data  ",
			wantUser: "www-data",
		},
		{
			name:     "empty defaults to root",
			input:    "",
			wantUser: "root",
		},
		{
			name:     "whitespace only defaults to root",
			input:    "   ",
			wantUser: "root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := createTestContainer("test-123")
			builder := NewCronJobBuilder("test-job", container)
			builder.SetUser(tt.input)

			if builder.user != tt.wantUser {
				t.Errorf("user = %q, want %q", builder.user, tt.wantUser)
			}
		})
	}
}

func TestCronJobBuilder_Build(t *testing.T) {
	container := createTestContainer("test-123")

	t.Run("successful build", func(t *testing.T) {
		builder := NewCronJobBuilder("test-job", container)
		builder.SetEnabled("true")
		builder.SetSchedule("@every 5m")
		builder.SetCommand("echo hello")
		builder.SetUser("www-data")

		job, err := builder.Build()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if job == nil {
			t.Fatal("expected job, got nil")
		}

		if job.Name != "test-job" {
			t.Errorf("job.Name = %q, want %q", job.Name, "test-job")
		}
		if job.ID != "test-123-test-job" {
			t.Errorf("job.ID = %q, want %q", job.ID, "test-123-test-job")
		}
		if !job.Enabled {
			t.Errorf("job.Enabled = false, want true")
		}
		if job.Schedule != "@every 5m" {
			t.Errorf("job.Schedule = %q, want %q", job.Schedule, "@every 5m")
		}
		if job.Command != "echo hello" {
			t.Errorf("job.Command = %q, want %q", job.Command, "echo hello")
		}
		if job.User != "www-data" {
			t.Errorf("job.User = %q, want %q", job.User, "www-data")
		}
		if job.Status != StatusIdle {
			t.Errorf("job.Status = %q, want %q", job.Status, StatusIdle)
		}
		if job.Container != container {
			t.Errorf("job.Container = %v, want %v", job.Container, container)
		}
	})

	t.Run("build with missing enabled", func(t *testing.T) {
		builder := NewCronJobBuilder("test-job", container)
		builder.SetSchedule("@every 5m")
		builder.SetCommand("echo hello")

		job, err := builder.Build()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if job != nil {
			t.Errorf("expected nil job, got %v", job)
		}
	})

	t.Run("build with missing schedule", func(t *testing.T) {
		builder := NewCronJobBuilder("test-job", container)
		builder.SetEnabled("true")
		builder.SetCommand("echo hello")

		job, err := builder.Build()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if job != nil {
			t.Errorf("expected nil job, got %v", job)
		}
	})

	t.Run("build with missing command", func(t *testing.T) {
		builder := NewCronJobBuilder("test-job", container)
		builder.SetEnabled("true")
		builder.SetSchedule("@every 5m")

		job, err := builder.Build()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if job != nil {
			t.Errorf("expected nil job, got %v", job)
		}
	})

	t.Run("build with validation errors", func(t *testing.T) {
		builder := NewCronJobBuilder("test-job", container)
		builder.SetEnabled("invalid")
		builder.SetSchedule("")
		builder.SetCommand("")

		job, err := builder.Build()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if job != nil {
			t.Errorf("expected nil job, got %v", job)
		}
		// Check that error message contains all validation errors
		errStr := err.Error()
		if errStr == "" {
			t.Error("expected non-empty error message")
		}
	})
}

func TestCronJobBuilder_IsValid(t *testing.T) {
	container := createTestContainer("test-123")

	t.Run("valid builder", func(t *testing.T) {
		builder := NewCronJobBuilder("test-job", container)
		builder.SetEnabled("true")
		builder.SetSchedule("@every 5m")
		builder.SetCommand("echo hello")

		if !builder.IsValid() {
			t.Errorf("expected valid, got invalid: %v", builder.GetErrors())
		}
	})

	t.Run("invalid builder with errors", func(t *testing.T) {
		builder := NewCronJobBuilder("test-job", container)
		builder.SetEnabled("invalid")

		if builder.IsValid() {
			t.Error("expected invalid, got valid")
		}
	})

	t.Run("invalid builder missing fields", func(t *testing.T) {
		builder := NewCronJobBuilder("test-job", container)

		if builder.IsValid() {
			t.Error("expected invalid, got valid")
		}
	})
}

func TestCronJobBuilder_Reset(t *testing.T) {
	container := createTestContainer("test-123")
	builder := NewCronJobBuilder("test-job", container)

	// Set values
	builder.SetEnabled("true")
	builder.SetSchedule("@every 5m")
	builder.SetCommand("echo hello")
	builder.SetUser("www-data")
	builder.addError("test error")

	// Reset
	builder.Reset()

	// Check that everything is reset
	if builder.enabled != nil {
		t.Errorf("enabled should be nil after reset, got %v", builder.enabled)
	}
	if builder.schedule != "" {
		t.Errorf("schedule should be empty after reset, got %q", builder.schedule)
	}
	if builder.command != "" {
		t.Errorf("command should be empty after reset, got %q", builder.command)
	}
	if builder.user != "root" {
		t.Errorf("user should be 'root' after reset, got %q", builder.user)
	}
	if builder.hasErrors {
		t.Error("hasErrors should be false after reset")
	}
	if len(builder.errors) > 0 {
		t.Errorf("errors should be empty after reset, got %v", builder.errors)
	}
}

func TestCronJobBuilder_GetErrors(t *testing.T) {
	container := createTestContainer("test-123")
	builder := NewCronJobBuilder("test-job", container)

	// Add multiple errors
	builder.addError("error 1")
	builder.addError("error 2")
	builder.addError("error 3")

	errors := builder.GetErrors()
	if len(errors) != 3 {
		t.Fatalf("expected 3 errors, got %d", len(errors))
	}

	// Check that it's a copy (modifying returned slice shouldn't affect builder)
	errors[0] = "modified"
	originalErrors := builder.GetErrors()
	if originalErrors[0] == "modified" {
		t.Error("GetErrors should return a copy, not the original slice")
	}
}

func TestCronJobBuilder_GetName(t *testing.T) {
	container := createTestContainer("test-123")
	builder := NewCronJobBuilder("my-test-job", container)

	if builder.GetName() != "my-test-job" {
		t.Errorf("GetName() = %q, want %q", builder.GetName(), "my-test-job")
	}
}

func TestCronJobBuilder_GetContainer(t *testing.T) {
	container := createTestContainer("test-123")
	builder := NewCronJobBuilder("test-job", container)

	if builder.GetContainer() != container {
		t.Errorf("GetContainer() = %v, want %v", builder.GetContainer(), container)
	}
}

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}

func compareBoolPtr(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// Test error accumulation
func TestCronJobBuilder_ErrorAccumulation(t *testing.T) {
	container := createTestContainer("test-123")
	builder := NewCronJobBuilder("test-job", container)

	// Accumulate multiple errors
	builder.SetEnabled("invalid")
	builder.SetSchedule("")
	builder.SetCommand("")

	if !builder.HasErrors() {
		t.Error("expected HasErrors() to be true")
	}

	errors := builder.GetErrors()
	if len(errors) != 3 {
		t.Errorf("expected 3 errors, got %d: %v", len(errors), errors)
	}

	// Validate should check for missing fields but doesn't duplicate existing errors
	builder.IsValid()
	errors = builder.GetErrors()
	// Since enabled is already invalid, we should still have the same errors
	// The builder correctly doesn't duplicate errors for already validated fields
	if len(errors) != 3 {
		t.Errorf("expected 3 errors after IsValid(), got %d: %v", len(errors), errors)
	}
}

// Test job ID generation
func TestCronJobBuilder_IDGeneration(t *testing.T) {
	tests := []struct {
		containerID string
		jobName     string
		expectedID  string
	}{
		{
			containerID: "abc123456789",
			jobName:     "backup",
			expectedID:  "abc123456789-backup",
		},
		{
			containerID: "xyz987654321",
			jobName:     "cleanup-logs",
			expectedID:  "xyz987654321-cleanup-logs",
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s-%s", tt.containerID, tt.jobName), func(t *testing.T) {
			container := &Container{ID: tt.containerID}
			builder := NewCronJobBuilder(tt.jobName, container)
			builder.SetEnabled("true")
			builder.SetSchedule("@every 1h")
			builder.SetCommand("echo test")

			job, err := builder.Build()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if job.ID != tt.expectedID {
				t.Errorf("job.ID = %q, want %q", job.ID, tt.expectedID)
			}
		})
	}
}
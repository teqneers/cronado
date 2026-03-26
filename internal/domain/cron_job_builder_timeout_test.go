package domain

import (
	"testing"
	"time"
)

func TestCronJobBuilder_SetTimeout(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantTimeout time.Duration
		wantError   bool
	}{
		{
			name:        "valid seconds",
			input:       "30s",
			wantTimeout: 30 * time.Second,
		},
		{
			name:        "valid minutes",
			input:       "5m",
			wantTimeout: 5 * time.Minute,
		},
		{
			name:        "valid hours",
			input:       "1h",
			wantTimeout: 1 * time.Hour,
		},
		{
			name:        "valid complex duration",
			input:       "1h30m",
			wantTimeout: 90 * time.Minute,
		},
		{
			name:        "whitespace trimming",
			input:       "  10m  ",
			wantTimeout: 10 * time.Minute,
		},
		{
			name:        "empty keeps default",
			input:       "",
			wantTimeout: DefaultTimeout,
		},
		{
			name:      "invalid format",
			input:     "notaduration",
			wantError: true,
		},
		{
			name:      "negative duration",
			input:     "-5s",
			wantError: true,
		},
		{
			name:      "zero duration",
			input:     "0s",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := createTestContainer("test-123")
			builder := NewCronJobBuilder("test-job", container)
			builder.SetTimeout(tt.input)

			if tt.wantError {
				if !builder.HasErrors() {
					t.Errorf("expected error for input %q", tt.input)
				}
			} else {
				if builder.HasErrors() {
					t.Errorf("unexpected error for input %q: %v", tt.input, builder.GetErrors())
				}
				if builder.timeout != tt.wantTimeout {
					t.Errorf("timeout = %v, want %v", builder.timeout, tt.wantTimeout)
				}
			}
		})
	}
}

func TestCronJobBuilder_Build_IncludesTimeout(t *testing.T) {
	container := createTestContainer("test-123")

	t.Run("default timeout", func(t *testing.T) {
		builder := NewCronJobBuilder("test-job", container)
		builder.SetEnabled("true")
		builder.SetSchedule("@every 5m")
		builder.SetCommand("echo hello")

		job, err := builder.Build()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if job.Timeout != DefaultTimeout {
			t.Errorf("job.Timeout = %v, want %v", job.Timeout, DefaultTimeout)
		}
	})

	t.Run("custom timeout", func(t *testing.T) {
		builder := NewCronJobBuilder("test-job", container)
		builder.SetEnabled("true")
		builder.SetSchedule("@every 5m")
		builder.SetCommand("echo hello")
		builder.SetTimeout("10m")

		job, err := builder.Build()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if job.Timeout != 10*time.Minute {
			t.Errorf("job.Timeout = %v, want %v", job.Timeout, 10*time.Minute)
		}
	})
}

func TestCronJobBuilder_Reset_IncludesTimeout(t *testing.T) {
	container := createTestContainer("test-123")
	builder := NewCronJobBuilder("test-job", container)

	builder.SetTimeout("10m")
	builder.Reset()

	if builder.timeout != DefaultTimeout {
		t.Errorf("timeout should be %v after reset, got %v", DefaultTimeout, builder.timeout)
	}
}

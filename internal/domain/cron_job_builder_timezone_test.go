package domain

import (
	"strings"
	"testing"
	"time"
)

func TestCronJobBuilder_SetTimezone(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		wantError bool
	}{
		{name: "IANA name", input: "Europe/Berlin", want: "Europe/Berlin"},
		{name: "UTC", input: "UTC", want: "UTC"},
		{name: "quoted with spaces", input: ` "America/New_York" `, want: "America/New_York"},
		{name: "empty keeps default", input: "", want: ""},
		{name: "unknown zone", input: "Mars/Olympus", wantError: true},
		{name: "offset is not a zone", input: "+02:00", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewCronJobBuilder("test-job", createTestContainer("test-123"), 0, 0)
			builder.SetTimezone(tt.input)

			if builder.HasErrors() != tt.wantError {
				t.Fatalf("HasErrors() = %v, want %v (errors: %v)", builder.HasErrors(), tt.wantError, builder.GetErrors())
			}
			if builder.timezone != tt.want {
				t.Errorf("timezone = %q, want %q", builder.timezone, tt.want)
			}
		})
	}
}

func TestCronJobBuilder_TimezoneConflictsWithScheduleZone(t *testing.T) {
	for _, schedule := range []string{"CRON_TZ=UTC 0 2 * * *", "TZ=UTC 0 2 * * *"} {
		t.Run(schedule, func(t *testing.T) {
			builder := NewCronJobBuilder("test-job", createTestContainer("test-123"), 0, 0)
			builder.SetEnabled("true")
			builder.SetSchedule(schedule)
			builder.SetCommand("echo hello")
			builder.SetTimezone("Europe/Berlin")

			_, err := builder.Build()
			if err == nil || !strings.Contains(err.Error(), "conflicts") {
				t.Fatalf("expected conflict error, got %v", err)
			}
		})
	}
}

func TestCronJob_SchedulerSpecAppliesTimezone(t *testing.T) {
	builder := NewCronJobBuilder("test-job", createTestContainer("test-123"), 0, 0)
	builder.SetEnabled("true")
	builder.SetSchedule("0 2 * * *")
	builder.SetCommand("echo hello")
	builder.SetTimezone("Europe/Berlin")

	job, err := builder.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job.Schedule != "0 2 * * *" {
		t.Errorf("Schedule = %q, want label value unchanged", job.Schedule)
	}
	if got, want := job.SchedulerSpec(), "CRON_TZ=Europe/Berlin 0 2 * * *"; got != want {
		t.Errorf("SchedulerSpec() = %q, want %q", got, want)
	}

	schedule, err := cronParser.Parse(job.SchedulerSpec())
	if err != nil {
		t.Fatalf("scheduler rejects spec: %v", err)
	}
	// 02:00 Berlin is 00:00 UTC in summer (CEST) and 01:00 UTC in winter (CET).
	summer := schedule.Next(time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)).UTC()
	if want := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC); !summer.Equal(want) {
		t.Errorf("summer next run = %s, want %s", summer, want)
	}
	winter := schedule.Next(time.Date(2026, 12, 1, 12, 0, 0, 0, time.UTC)).UTC()
	if want := time.Date(2026, 12, 2, 1, 0, 0, 0, time.UTC); !winter.Equal(want) {
		t.Errorf("winter next run = %s, want %s", winter, want)
	}
}

func TestCronJob_SchedulerSpecWithoutTimezone(t *testing.T) {
	job := CronJob{Schedule: "@every 5m"}
	if got := job.SchedulerSpec(); got != "@every 5m" {
		t.Errorf("SchedulerSpec() = %q, want %q", got, "@every 5m")
	}
}

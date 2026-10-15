package domain

import (
	"reflect"
	"testing"
)

func TestParseJobLabels_ReportsAllIssues(t *testing.T) {
	container := &Container{
		ID:   "abc123456789",
		Name: "app",
		Labels: map[string]string{
			"cronado.ok.enabled":   "true",
			"cronado.ok.schedule":  "@every 1h",
			"cronado.ok.cmd":       "echo ok",
			"cronado.ok.timout":    "5m",
			"cronado.bad.enabled":  "maybe",
			"cronado.bad.cmd":      "echo bad",
			"cronado.broken":       "x",
			"cronado.no space.cmd": "echo",
			"cronado.off.enabled":  "false",
			"cronado.off.schedule": "@daily",
			"cronado.off.cmd":      "echo off",
			"other.label":          "ignored",
			"cronadoextra.x.y":     "ignored",
		},
	}

	jobs, issues := ParseJobLabels(container, "cronado", 0, 0)

	if len(jobs) != 2 || !jobs["ok"].Enabled || jobs["off"].Enabled {
		t.Errorf("jobs = %v, want enabled ok and disabled off", jobs)
	}

	want := []LabelIssue{
		{Label: "cronado.broken", Message: "invalid label format, expected cronado.<job>.<property>"},
		{Job: "bad", Message: "invalid enabled value: maybe (expected true/false)", JobRejected: true},
		{Job: "bad", Message: "schedule not set", JobRejected: true},
		{Job: "no space", Label: "cronado.no space.cmd", Message: "invalid job name, must match [a-zA-Z0-9][a-zA-Z0-9_-]*"},
		{Job: "ok", Label: "cronado.ok.timout", Message: `unknown property "timout", valid properties are ` + validProperties},
	}
	if !reflect.DeepEqual(issues, want) {
		t.Errorf("issues =\n%#v\nwant\n%#v", issues, want)
	}
}

func TestParseJobLabels_NoLabels(t *testing.T) {
	jobs, issues := ParseJobLabels(&Container{Labels: map[string]string{"app": "x"}}, "cronado", 0, 0)
	if len(jobs) != 0 || len(issues) != 0 {
		t.Errorf("jobs = %v, issues = %v, want none", jobs, issues)
	}
}

func TestCronJobBuilder_InvalidLabelNotReportedAsMissing(t *testing.T) {
	builder := NewCronJobBuilder("job", createTestContainer("test-123"), 0, 0)
	builder.SetEnabled("maybe")
	builder.SetSchedule("@every nope")
	builder.SetCommand("")

	if builder.IsValid() {
		t.Fatal("expected invalid")
	}
	want := []string{
		"invalid enabled value: maybe (expected true/false)",
		"invalid @every duration: nope",
		"command cannot be empty",
	}
	if got := builder.GetErrors(); !reflect.DeepEqual(got, want) {
		t.Errorf("errors = %q, want %q", got, want)
	}
}

func TestParseJobLabels_TimezoneAndJitter(t *testing.T) {
	container := &Container{
		ID:   "abc123456789",
		Name: "app",
		Labels: map[string]string{
			"cronado.report.enabled":  "true",
			"cronado.report.schedule": "0 * * * *",
			"cronado.report.cmd":      "php report.php",
			"cronado.report.timezone": "Europe/Berlin",
			"cronado.report.jitter":   "5m",
			"cronado.bad.enabled":     "true",
			"cronado.bad.schedule":    "@every 10m",
			"cronado.bad.cmd":         "echo",
			"cronado.bad.timezone":    "Mars/Olympus",
			"cronado.bad.jitter":      "10m",
		},
	}

	jobs, issues := ParseJobLabels(container, "cronado", 0, 0)

	report, ok := jobs["report"]
	if !ok {
		t.Fatalf("report job missing, issues: %v", issues)
	}
	if report.Timezone != "Europe/Berlin" || report.Jitter.String() != "5m0s" {
		t.Errorf("report timezone = %q, jitter = %s", report.Timezone, report.Jitter)
	}

	var badMessages []string
	for _, issue := range issues {
		if issue.Job != "bad" || !issue.JobRejected {
			t.Errorf("unexpected issue: %+v", issue)
		}
		badMessages = append(badMessages, issue.Message)
	}
	want := []string{
		"invalid timezone: Mars/Olympus (expected IANA name like Europe/Berlin)",
		"jitter 10m0s must be shorter than the schedule interval 10m0s",
	}
	if !reflect.DeepEqual(badMessages, want) {
		t.Errorf("bad job messages = %q, want %q", badMessages, want)
	}
}

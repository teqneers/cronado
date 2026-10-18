package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/teqneers/cronado/internal/config"
	"github.com/teqneers/cronado/internal/domain"
)

func target(name string, labels map[string]string) validateTarget {
	return validateTarget{name: name, container: &domain.Container{Name: name, Labels: labels}}
}

func TestValidateTargets_ReportsProblems(t *testing.T) {
	targets := []validateTarget{
		target("db", map[string]string{
			"cronado.backup.enabled":  "true",
			"cronado.backup.schedule": "0 2 * * *",
			"cronado.backup.cmd":      "pg_dump",
		}),
		target("app", map[string]string{
			"cronado.report.enabled":  "true",
			"cronado.report.schedule": "0 25 * * *",
			"cronado.report.cmd":      "php report.php",
			"cronado.report.timout":   "5m",
		}),
		target("web", map[string]string{"other": "label"}),
	}

	var out bytes.Buffer
	if validateTargets(&out, targets, nil, "cronado", 0, 0) {
		t.Error("expected failure")
	}

	got := out.String()
	for _, want := range []string{
		"db\n  ✓ backup: 0 2 * * * (enabled)\n",
		"app\n",
		`  ✗ report: invalid cron schedule "0 25 * * *"`,
		`  ! cronado.report.timout: unknown property "timout"`,
		"Checked 2 containers: 1 job OK, 1 job rejected, 1 label ignored.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "web") {
		t.Errorf("container without cronado labels should be skipped:\n%s", got)
	}
}

func TestValidateTargets_AllValid(t *testing.T) {
	targets := []validateTarget{target("db", map[string]string{
		"cronado.backup.enabled":  "false",
		"cronado.backup.schedule": "@daily",
		"cronado.backup.cmd":      "pg_dump",
	})}

	var out bytes.Buffer
	if !validateTargets(&out, targets, nil, "cronado", 0, 0) {
		t.Errorf("expected success:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "✓ backup: @daily (disabled)") {
		t.Errorf("disabled job not listed:\n%s", out.String())
	}
}

func TestValidateTargets_NothingFound(t *testing.T) {
	var out bytes.Buffer
	if !validateTargets(&out, []validateTarget{target("web", nil)}, nil, "cronado", 0, 0) {
		t.Error("expected success when no labels found")
	}
	if !strings.Contains(out.String(), "No containers with cronado.* labels found.") {
		t.Errorf("output = %q", out.String())
	}
}

func TestValidateTargets_AcceptsTimezoneAndJitter(t *testing.T) {
	targets := []validateTarget{target("app", map[string]string{
		"cronado.report.enabled":  "true",
		"cronado.report.schedule": "0 * * * *",
		"cronado.report.cmd":      "php report.php",
		"cronado.report.timezone": "Europe/Berlin",
		"cronado.report.jitter":   "5m",
	})}

	var out bytes.Buffer
	if !validateTargets(&out, targets, nil, "cronado", 0, 0) {
		t.Errorf("expected success:\n%s", out.String())
	}
}

func TestValidateTargets_ConfigJobs(t *testing.T) {
	configJobs := []config.JobConfig{
		{Name: "backup", Container: "db", Schedule: "0 2 * * *", Cmd: "pg_dump"},
		{Name: "report", Container: "worker", Schedule: "0 25 * * *", Cmd: "php report.php"},
		{Name: "orphan"},
		{Name: "offsite", Image: "restic/restic:0.17.3", Schedule: "0 3 * * *", Cmd: "backup /data"},
	}
	targets := addConfigJobTargets([]validateTarget{target("db", nil)}, configJobs)

	var out bytes.Buffer
	if validateTargets(&out, targets, configJobs, "cronado", 0, 0) {
		t.Error("validateTargets = true, want false")
	}
	got := out.String()
	for _, want := range []string{
		"config file\n  ✗ jobs[2] orphan: container or image is required\n",
		"own containers (image)\n  ✓ offsite: 0 3 * * * (enabled, config file)\n",
		"db\n  ✓ backup: 0 2 * * * (enabled, config file)\n",
		"worker (not running)\n  ✗ report: invalid cron schedule",
		"(config file)\n",
		"Checked 3 containers: 2 jobs OK, 1 job rejected, 0 labels ignored.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

func TestFormatLabelIssue_ConfigSource(t *testing.T) {
	got := formatLabelIssue(domain.ContainerLabelIssue{Job: "backup", Message: "command not set", JobRejected: true, Source: domain.SourceConfig})
	if want := "job backup rejected: command not set (config file)"; got != want {
		t.Errorf("formatLabelIssue = %q, want %q", got, want)
	}
}

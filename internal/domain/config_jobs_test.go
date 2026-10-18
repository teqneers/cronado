package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/teqneers/cronado/internal/config"
	cronadoCtx "github.com/teqneers/cronado/internal/context"
)

func TestParseContainerJobs_ConfigJobsForMatchingContainer(t *testing.T) {
	disabled := false
	db := &Container{ID: "aaaaaaaaaaaa1", Name: "db", Labels: map[string]string{
		"cronado.vacuum.enabled":  "true",
		"cronado.vacuum.schedule": "0 4 * * *",
		"cronado.vacuum.cmd":      "vacuumdb --all",
	}}
	jobs := []config.JobConfig{
		{Name: "backup", Container: "db", Schedule: "0 2 * * *", Cmd: "pg_dump mydb", User: "postgres", Timeout: "10m", Jitter: "5m"},
		{Name: "paused", Container: "db", Enabled: &disabled, Schedule: "0 3 * * *", Cmd: "true"},
		{Name: "other", Container: "web", Schedule: "0 2 * * *", Cmd: "true"},
	}

	parsed, issues := ParseContainerJobs(db, "cronado", jobs, time.Minute, time.Hour)
	if len(issues) != 0 {
		t.Fatalf("issues = %+v", issues)
	}
	if len(parsed) != 3 {
		t.Fatalf("jobs = %v, want vacuum, backup and paused", parsed)
	}

	backup := parsed["backup"]
	if backup.Source != SourceConfig || backup.Container != db || backup.ID != "aaaaaaaaaaaa-backup" ||
		backup.Command != "pg_dump mydb" || backup.User != "postgres" || backup.Timeout != 10*time.Minute ||
		backup.Jitter != 5*time.Minute || !backup.Enabled {
		t.Errorf("backup = %+v", backup)
	}
	if parsed["paused"].Enabled {
		t.Error("job with enabled: false is enabled")
	}
	if parsed["vacuum"].Source != SourceLabel {
		t.Errorf("label job source = %q", parsed["vacuum"].Source)
	}
}

func TestParseContainerJobs_InvalidConfigJobIsRejected(t *testing.T) {
	db := &Container{ID: "aaaaaaaaaaaa1", Name: "db"}
	jobs := []config.JobConfig{{Name: "backup", Container: "db", Schedule: "0 25 * * *"}}

	parsed, issues := ParseContainerJobs(db, "cronado", jobs, time.Minute, time.Hour)
	if len(parsed) != 0 {
		t.Errorf("jobs = %v, want none", parsed)
	}
	var messages []string
	for _, issue := range issues {
		if !issue.JobRejected || issue.Source != SourceConfig || issue.Job != "backup" {
			t.Errorf("issue = %+v", issue)
		}
		messages = append(messages, issue.Message)
	}
	joined := strings.Join(messages, "; ")
	if !strings.Contains(joined, "invalid cron schedule") || !strings.Contains(joined, "command not set") {
		t.Errorf("messages = %q, want invalid schedule and missing cmd", joined)
	}
}

func TestParseContainerJobs_LabelWinsOverConfigJob(t *testing.T) {
	db := &Container{ID: "aaaaaaaaaaaa1", Name: "db", Labels: map[string]string{
		"cronado.backup.enabled":  "true",
		"cronado.backup.schedule": "0 2 * * *",
		"cronado.backup.cmd":      "from-label",
	}}
	jobs := []config.JobConfig{{Name: "backup", Container: "db", Schedule: "0 3 * * *", Cmd: "from-config"}}

	parsed, issues := ParseContainerJobs(db, "cronado", jobs, time.Minute, time.Hour)
	if parsed["backup"].Command != "from-label" || parsed["backup"].Source != SourceLabel {
		t.Errorf("backup = %+v, want the label job", parsed["backup"])
	}
	if len(issues) != 1 || issues[0].Source != SourceConfig || !issues[0].JobRejected ||
		!strings.Contains(issues[0].Message, "also defined by cronado.backup.* labels") {
		t.Errorf("issues = %+v", issues)
	}
}

func TestParseContainerJobs_ConfigJobUsesReplicaLabelsOfContainer(t *testing.T) {
	web := composeReplica("aaaaaaaaaaaa1", "web", "1")
	jobs := []config.JobConfig{{Name: "cleanup", Container: web.Name, Schedule: "0 2 * * *", Cmd: "true", Replicas: "all"}}

	parsed, issues := ParseContainerJobs(web, "cronado", jobs, time.Minute, time.Hour)
	if len(issues) != 0 {
		t.Fatalf("issues = %+v", issues)
	}
	job := parsed["cleanup"]
	if job.Replicas != ReplicasAll {
		t.Errorf("Replicas = %q", job.Replicas)
	}
	if _, _, ok := job.Container.ComposeService(); !ok {
		t.Error("config job lost the Compose labels of its container")
	}
}

func TestCheckConfigJobs(t *testing.T) {
	problems := CheckConfigJobs([]config.JobConfig{
		{Name: "ok", Container: "db"},
		{Container: "db"},
		{Name: "bad name", Container: "db"},
		{Name: "nocontainer"},
		{Name: "ok", Container: "db"},
		{Name: "ok", Container: "web"},
	})
	want := []string{
		"jobs[1]: name is required",
		"jobs[2] bad name: invalid job name",
		"jobs[3] nocontainer: container or image is required",
		"jobs[4] ok: defined more than once for container db",
	}
	if len(problems) != len(want) {
		t.Fatalf("problems = %q", problems)
	}
	for i, prefix := range want {
		if !strings.HasPrefix(problems[i], prefix) {
			t.Errorf("problems[%d] = %q, want prefix %q", i, problems[i], prefix)
		}
	}
}

func TestHandleContainer_RegistersConfigJobs(t *testing.T) {
	original := cronadoCtx.AppCtx
	defer func() { cronadoCtx.AppCtx = original }()

	manager, _, _, _, _ := createTestCronJobManager()
	cronadoCtx.AppCtx = &cronadoCtx.AppContext{
		CronJobManager: manager,
		Config: &config.Config{CronLabelPrefix: "cronado", Jobs: []config.JobConfig{
			{Name: "backup", Container: "db", Schedule: "0 2 * * *", Cmd: "pg_dump"},
			{Name: "report", Container: "db", Schedule: "0 25 * * *", Cmd: "true"},
		}},
	}

	// A container without any cron labels gets the config jobs.
	handleContainer(&Container{ID: "abc123456789", Name: "db", Status: "running"})

	all := manager.GetAllTyped()
	if len(all) != 1 || all[0].Name != "backup" || all[0].Source != SourceConfig {
		t.Fatalf("jobs = %+v, want the backup config job", all)
	}
	issues := manager.GetLabelIssues()
	if len(issues) != 1 || issues[0].Job != "report" || issues[0].Source != SourceConfig {
		t.Errorf("issues = %+v, want the report config job", issues)
	}
}

package domain

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/teqneers/cronado/internal/config"
	cronadoCtx "github.com/teqneers/cronado/internal/context"
)

func TestCronJobManager_LabelIssues(t *testing.T) {
	manager, _, _, _, _ := createTestCronJobManager()
	web := &Container{ID: "bbb", Name: "web"}
	app := &Container{ID: "aaa", Name: "app"}

	manager.SetLabelIssues(web, []LabelIssue{{Job: "report", Message: "schedule not set", JobRejected: true}})
	manager.SetLabelIssues(app, []LabelIssue{{Job: "x", Label: "cronado.x.timout", Message: "unknown property"}})

	issues := manager.GetLabelIssues()
	if len(issues) != 2 || issues[0].ContainerName != "app" || issues[1].ContainerName != "web" {
		t.Fatalf("issues = %+v, want app then web", issues)
	}
	if got := issues[1]; got.ContainerID != "bbb" || got.Job != "report" || !got.JobRejected {
		t.Errorf("web issue = %+v", got)
	}

	// Fixed labels clear the issues of a container.
	manager.SetLabelIssues(web, nil)
	if issues := manager.GetLabelIssues(); len(issues) != 1 || issues[0].ContainerName != "app" {
		t.Errorf("after clearing web: %+v", issues)
	}

	// A stopped container's issues are removed with its jobs.
	manager.Remove("aaa")
	if issues := manager.GetLabelIssues(); len(issues) != 0 {
		t.Errorf("after Remove: %+v", issues)
	}
}

func TestCronJobManager_RetainLabelIssues(t *testing.T) {
	manager, _, _, _, _ := createTestCronJobManager()
	issue := []LabelIssue{{Job: "x", Message: "schedule not set", JobRejected: true}}
	manager.SetLabelIssues(&Container{ID: "running", Name: "a"}, issue)
	manager.SetLabelIssues(&Container{ID: "gone", Name: "b"}, issue)

	manager.RetainLabelIssues(map[string]bool{"running": true})

	issues := manager.GetLabelIssues()
	if len(issues) != 1 || issues[0].ContainerID != "running" {
		t.Errorf("issues = %+v, want only the running container", issues)
	}
}

func TestHandleContainer_RecordsLabelIssues(t *testing.T) {
	original := cronadoCtx.AppCtx
	defer func() { cronadoCtx.AppCtx = original }()

	manager, _, _, _, _ := createTestCronJobManager()
	cronadoCtx.AppCtx = &cronadoCtx.AppContext{
		CronJobManager: manager,
		Config:         &config.Config{CronLabelPrefix: "cronado"},
	}

	// Only an invalid job: nothing is registered, but the problem is kept.
	container := &Container{
		ID:   "abc123456789",
		Name: "app",
		Labels: map[string]string{
			"cronado.report.enabled":  "true",
			"cronado.report.schedule": "0 25 * * *",
			"cronado.report.cmd":      "php report.php",
		},
	}
	handleContainer(container)

	if manager.GetJobCount() != 0 {
		t.Errorf("jobs = %d, want 0", manager.GetJobCount())
	}
	issues := manager.GetLabelIssues()
	if len(issues) != 1 || issues[0].Job != "report" || !strings.Contains(issues[0].Message, "invalid cron schedule") {
		t.Fatalf("issues = %+v", issues)
	}
}

func TestJobStateCollector_LabelIssues(t *testing.T) {
	manager, _, _, _, _ := createTestCronJobManager()
	manager.SetLabelIssues(&Container{ID: "abcdef1234567890", Name: "app"}, []LabelIssue{
		{Job: "report", Message: "invalid enabled value", JobRejected: true},
		{Job: "report", Message: "schedule not set", JobRejected: true},
		{Job: "report", Label: "cronado.report.timout", Message: "unknown property"},
	})

	expected := `
# HELP cronado_label_issues Number of problems in the cron job labels of a running container; kind is rejected_job or ignored_label
# TYPE cronado_label_issues gauge
cronado_label_issues{container_id="abcdef123456",container_name="app",job_name="report",kind="ignored_label"} 1
cronado_label_issues{container_id="abcdef123456",container_name="app",job_name="report",kind="rejected_job"} 2
`
	if err := testutil.CollectAndCompare(NewJobStateCollector(manager), strings.NewReader(expected), "cronado_label_issues"); err != nil {
		t.Error(err)
	}
}

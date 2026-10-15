package domain

import (
	"github.com/prometheus/client_golang/prometheus"
)

var jobStateLabels = []string{"container_id", "container_name", "job_name"}

var (
	lastRunDesc = prometheus.NewDesc("cronado_job_last_run_timestamp_seconds",
		"Unix time of the last run of a cron job", jobStateLabels, nil)
	lastSuccessDesc = prometheus.NewDesc("cronado_job_last_success_timestamp_seconds",
		"Unix time of the last successful run of a cron job", jobStateLabels, nil)
	nextRunDesc = prometheus.NewDesc("cronado_job_next_run_timestamp_seconds",
		"Unix time of the next scheduled run of a cron job", jobStateLabels, nil)
	runningDesc = prometheus.NewDesc("cronado_job_running",
		"1 if the cron job is currently running, 0 otherwise", jobStateLabels, nil)
	labelIssuesDesc = prometheus.NewDesc("cronado_label_issues",
		"Number of problems in the cron job labels of a running container; kind is rejected_job or ignored_label",
		[]string{"container_id", "container_name", "job_name", "kind"}, nil)
)

// JobStateCollector exposes the current state of every registered job. It
// reads the job list on each scrape, so removed jobs disappear automatically.
type JobStateCollector struct {
	manager *CronJobManager
}

// NewJobStateCollector creates a collector for the jobs of manager.
func NewJobStateCollector(manager *CronJobManager) *JobStateCollector {
	return &JobStateCollector{manager: manager}
}

func (c *JobStateCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- lastRunDesc
	ch <- lastSuccessDesc
	ch <- nextRunDesc
	ch <- runningDesc
	ch <- labelIssuesDesc
}

func (c *JobStateCollector) Collect(ch chan<- prometheus.Metric) {
	for _, job := range c.manager.GetAllTyped() {
		containerName := ""
		if job.Container != nil {
			containerName = job.Container.Name
		}
		labels := []string{job.GetShortContainerID(), containerName, job.Name}

		if job.LastRun != nil {
			ch <- prometheus.MustNewConstMetric(lastRunDesc, prometheus.GaugeValue, float64(job.LastRun.Unix()), labels...)
		}
		if job.LastSuccess != nil {
			ch <- prometheus.MustNewConstMetric(lastSuccessDesc, prometheus.GaugeValue, float64(job.LastSuccess.Unix()), labels...)
		}
		if job.NextRun != nil {
			ch <- prometheus.MustNewConstMetric(nextRunDesc, prometheus.GaugeValue, float64(job.NextRun.Unix()), labels...)
		}
		running := 0.0
		if job.IsRunning() {
			running = 1
		}
		ch <- prometheus.MustNewConstMetric(runningDesc, prometheus.GaugeValue, running, labels...)
	}

	type issueKey struct{ containerID, containerName, job, kind string }
	counts := make(map[issueKey]int)
	for _, issue := range c.manager.GetLabelIssues() {
		kind := "ignored_label"
		if issue.JobRejected {
			kind = "rejected_job"
		}
		shortID := issue.ContainerID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		counts[issueKey{shortID, issue.ContainerName, issue.Job, kind}]++
	}
	for k, n := range counts {
		ch <- prometheus.MustNewConstMetric(labelIssuesDesc, prometheus.GaugeValue, float64(n), k.containerID, k.containerName, k.job, k.kind)
	}
}

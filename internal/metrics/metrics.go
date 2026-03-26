package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// JobExecCounter counts the total number of cron job executions, labeled by container, job name, and status.
var JobExecCounter *prometheus.CounterVec

// JobExecDuration records the duration of cron job executions in seconds, labeled by container and job name.
var JobExecDuration *prometheus.HistogramVec

// ScheduledJobsGauge tracks the current number of scheduled cron jobs.
var ScheduledJobsGauge prometheus.Gauge

// Init registers and initializes all Prometheus metrics.
func Init() {
	JobExecCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "cronado",
			Name:      "job_executions_total",
			Help:      "Total number of cron job executions",
		},
		[]string{"container_id", "job_name", "status"},
	)
	JobExecDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "cronado",
			Name:      "job_duration_seconds",
			Help:      "Duration of cron job execution in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"container_id", "job_name"},
	)
	ScheduledJobsGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "cronado",
			Name:      "scheduled_jobs",
			Help:      "Current number of scheduled cron jobs",
		},
	)
	// Register metrics with the default registry
	prometheus.MustRegister(JobExecCounter, JobExecDuration, ScheduledJobsGauge)
}

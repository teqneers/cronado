package domain

import (
	"github.com/teqneers/cronado/internal/metrics"
)

// MetricsCollector defines the interface for collecting metrics
type MetricsCollector interface {
	IncrementScheduledJobs()
	DecrementScheduledJobs()
	RecordJobExecution(containerID, jobName, status string)
	RecordJobDuration(containerID, jobName string, duration float64)
}

// PrometheusMetricsCollector adapts the metrics package to the MetricsCollector interface
type PrometheusMetricsCollector struct{}

// NewPrometheusMetricsCollector creates a new PrometheusMetricsCollector
func NewPrometheusMetricsCollector() *PrometheusMetricsCollector {
	return &PrometheusMetricsCollector{}
}

// IncrementScheduledJobs increments the scheduled jobs gauge
func (c *PrometheusMetricsCollector) IncrementScheduledJobs() {
	if metrics.ScheduledJobsGauge != nil {
		metrics.ScheduledJobsGauge.Inc()
	}
}

// DecrementScheduledJobs decrements the scheduled jobs gauge
func (c *PrometheusMetricsCollector) DecrementScheduledJobs() {
	if metrics.ScheduledJobsGauge != nil {
		metrics.ScheduledJobsGauge.Dec()
	}
}

// RecordJobExecution records a job execution with the given status
func (c *PrometheusMetricsCollector) RecordJobExecution(containerID, jobName, status string) {
	if metrics.JobExecCounter != nil {
		metrics.JobExecCounter.WithLabelValues(containerID, jobName, status).Inc()
	}
}

// RecordJobDuration records the duration of a job execution
func (c *PrometheusMetricsCollector) RecordJobDuration(containerID, jobName string, duration float64) {
	if metrics.JobExecDuration != nil {
		metrics.JobExecDuration.WithLabelValues(containerID, jobName).Observe(duration)
	}
}
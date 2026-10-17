package domain

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// validJobName matches alphanumeric names with hyphens and underscores
var validJobName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// validProperties lists the job properties that can be set via labels.
const validProperties = "enabled,schedule,cmd,user,timezone,timeout,jitter,ping_url,replicas"

// LabelIssue describes a problem found in a container's cron job labels.
type LabelIssue struct {
	Job     string // job name, empty if the label could not be assigned to a job
	Label   string // offending label, empty for problems with the job as a whole
	Message string
	// JobRejected is true if the job is not scheduled because of this issue.
	// Otherwise the label is ignored and the job may still run.
	JobRejected bool
	// Source is SourceConfig for jobs from the config file, empty for labels.
	Source string
}

// ParseJobLabels parses the cron job labels of container. It returns every
// job that could be built, including disabled ones, and all issues found.
// Issues are sorted by job and label so output is stable.
func ParseJobLabels(container *Container, prefix string, minScheduleInterval, maxTimeout time.Duration) (map[string]CronJob, []LabelIssue) {
	builders := make(map[string]*CronJobBuilder)
	var issues []LabelIssue

	prefixWithDot := prefix + "."

	labels := make([]string, 0, len(container.Labels))
	for label := range container.Labels {
		if strings.HasPrefix(label, prefixWithDot) {
			labels = append(labels, label)
		}
	}
	sort.Strings(labels)

	for _, label := range labels {
		labelValue := container.Labels[label]
		parts := strings.SplitN(strings.TrimPrefix(label, prefixWithDot), ".", 2)

		if len(parts) != 2 {
			issues = append(issues, LabelIssue{
				Label:   label,
				Message: fmt.Sprintf("invalid label format, expected %s.<job>.<property>", prefix),
			})
			continue
		}

		jobName := parts[0]
		property := parts[1]

		if !validJobName.MatchString(jobName) {
			issues = append(issues, LabelIssue{
				Job:     jobName,
				Label:   label,
				Message: "invalid job name, must match [a-zA-Z0-9][a-zA-Z0-9_-]*",
			})
			continue
		}

		builder, exists := builders[jobName]
		if !exists {
			builder = NewCronJobBuilder(jobName, container, minScheduleInterval, maxTimeout)
			builders[jobName] = builder
		}

		switch property {
		case "enabled":
			builder.SetEnabled(labelValue)
		case "schedule":
			builder.SetSchedule(labelValue)
		case "cmd":
			builder.SetCommand(labelValue)
		case "user":
			builder.SetUser(labelValue)
		case "timezone":
			builder.SetTimezone(labelValue)
		case "timeout":
			builder.SetTimeout(labelValue)
		case "jitter":
			builder.SetJitter(labelValue)
		case "ping_url":
			builder.SetPingURL(labelValue)
		case "replicas":
			builder.SetReplicas(labelValue)
		default:
			issues = append(issues, LabelIssue{
				Job:     jobName,
				Label:   label,
				Message: fmt.Sprintf("unknown property %q, valid properties are %s", property, validProperties),
			})
		}
	}

	jobs := make(map[string]CronJob, len(builders))
	for jobName, builder := range builders {
		job, err := builder.Build()
		if err != nil {
			for _, msg := range builder.GetErrors() {
				issues = append(issues, LabelIssue{Job: jobName, Message: msg, JobRejected: true})
			}
			continue
		}
		jobs[jobName] = *job
	}

	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Job != issues[j].Job {
			return issues[i].Job < issues[j].Job
		}
		return issues[i].Label < issues[j].Label
	})

	return jobs, issues
}

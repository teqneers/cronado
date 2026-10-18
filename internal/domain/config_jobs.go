package domain

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/teqneers/cronado/internal/config"
)

// Sources of a cron job definition
const (
	SourceLabel  = "label"
	SourceConfig = "config"
)

// configJobPrefix is the label prefix config jobs are translated to, so that
// they are parsed with the same rules as labels.
const configJobPrefix = "config"

// configRunOwner owns the config file jobs that have an image instead of a
// container. It is not a Docker container: its jobs are registered at
// startup and stay for as long as cronado runs. Its empty name matches the
// empty container of those jobs.
var configRunOwner = &Container{ID: "cronado"}

// ConfigRunOwner returns the owner of the config file jobs with an image.
func ConfigRunOwner() *Container {
	return configRunOwner
}

// CheckConfigJobs returns the problems of the config file jobs that do not
// depend on a container: missing names or targets and duplicates.
func CheckConfigJobs(jobs []config.JobConfig) []string {
	var problems []string
	seen := make(map[string]bool)
	for i, job := range jobs {
		jobProblems := checkConfigJob(i, job)
		problems = append(problems, jobProblems...)
		if len(jobProblems) > 0 {
			continue
		}
		name := strings.TrimSpace(job.Name)
		key := strings.TrimSpace(job.Container) + "/" + name
		if seen[key] {
			if job.Container == "" {
				problems = append(problems, fmt.Sprintf("jobs[%d] %s: defined more than once", i, name))
			} else {
				problems = append(problems, fmt.Sprintf("jobs[%d] %s: defined more than once for container %s", i, name, strings.TrimSpace(job.Container)))
			}
		}
		seen[key] = true
	}
	return problems
}

// checkConfigJob returns the problems of a single config job that keep it
// from being parsed.
func checkConfigJob(i int, job config.JobConfig) []string {
	var problems []string
	name := strings.TrimSpace(job.Name)
	container := strings.TrimSpace(job.Container)
	image := strings.TrimSpace(job.Image)
	switch {
	case name == "":
		problems = append(problems, fmt.Sprintf("jobs[%d]: name is required", i))
	case !validJobName.MatchString(name):
		problems = append(problems, fmt.Sprintf("jobs[%d] %s: invalid job name, must match [a-zA-Z0-9][a-zA-Z0-9_-]*", i, name))
	}
	switch {
	case container == "" && image == "":
		problems = append(problems, fmt.Sprintf("jobs[%d] %s: container or image is required", i, name))
	case container != "" && image != "":
		problems = append(problems, fmt.Sprintf("jobs[%d] %s: set either container or image, not both", i, name))
	}
	for _, entry := range job.Environment {
		if key, _, ok := strings.Cut(entry, "="); !ok || key == "" {
			problems = append(problems, fmt.Sprintf("jobs[%d] %s: environment entry %q must be KEY=value", i, name, entry))
		}
	}
	return problems
}

// ParseConfigJobs parses the config file jobs that target container, with
// the same rules as labels. Jobs with problems found by CheckConfigJobs are
// skipped. The jobs with an image instead of a container are those of
// ConfigRunOwner.
func ParseConfigJobs(container *Container, jobs []config.JobConfig, minScheduleInterval, maxTimeout time.Duration) (map[string]CronJob, []LabelIssue) {
	labels := make(map[string]string)
	seen := make(map[string]bool)
	for i, job := range jobs {
		name := strings.TrimSpace(job.Name)
		if strings.TrimSpace(job.Container) != container.Name || len(checkConfigJob(i, job)) > 0 || seen[name] {
			continue
		}
		seen[name] = true
		enabled := "true"
		if job.Enabled != nil && !*job.Enabled {
			enabled = "false"
		}
		for property, value := range map[string]string{
			"enabled":  enabled,
			"schedule": job.Schedule,
			"cmd":      job.Cmd,
			"user":     job.User,
			"timezone": job.Timezone,
			"timeout":  job.Timeout,
			"jitter":   job.Jitter,
			"ping_url": job.PingURL,
			"replicas": job.Replicas,
			"image":    job.Image,
			"network":  job.Network,
			"volumes":  strings.Join(job.Volumes, ","),
		} {
			if value != "" {
				labels[configJobPrefix+"."+name+"."+property] = value
			}
		}
		for _, entry := range job.Environment {
			key, value, _ := strings.Cut(entry, "=")
			labels[configJobPrefix+"."+name+".env."+key] = value
		}
	}
	if len(labels) == 0 {
		return nil, nil
	}

	// Parse with a copy that carries the translated labels, but point the
	// jobs at the real container, whose labels decide about replicas.
	translated := *container
	translated.Labels = labels
	parsed, issues := ParseJobLabels(&translated, configJobPrefix, minScheduleInterval, maxTimeout)
	for name, job := range parsed {
		job.Container = container
		job.Source = SourceConfig
		parsed[name] = job
	}
	for i := range issues {
		issues[i].Source = SourceConfig
	}
	return parsed, issues
}

// ParseContainerJobs returns the jobs of container from its labels and from
// the config file. A config job with the same name as a label job is
// rejected, so the label wins.
func ParseContainerJobs(container *Container, prefix string, configJobs []config.JobConfig, minScheduleInterval, maxTimeout time.Duration) (map[string]CronJob, []LabelIssue) {
	jobs, issues := ParseJobLabels(container, prefix, minScheduleInterval, maxTimeout)
	for name, job := range jobs {
		job.Source = SourceLabel
		jobs[name] = job
	}
	labelJobs := labelJobNames(container, prefix)

	fromConfig, configIssues := ParseConfigJobs(container, configJobs, minScheduleInterval, maxTimeout)
	for _, issue := range configIssues {
		if !labelJobs[issue.Job] {
			issues = append(issues, issue)
		}
	}
	for name, job := range fromConfig {
		if labelJobs[name] {
			continue
		}
		jobs[name] = job
	}
	for name := range configJobNames(container, configJobs) {
		if labelJobs[name] {
			issues = append(issues, LabelIssue{
				Job:         name,
				Message:     fmt.Sprintf("job %s is also defined by %s.%s.* labels on this container, the labels are used", name, prefix, name),
				JobRejected: true,
				Source:      SourceConfig,
			})
		}
	}
	return jobs, issues
}

// labelJobNames returns the names of the jobs that labels of container
// define, valid or not.
func labelJobNames(container *Container, prefix string) map[string]bool {
	names := make(map[string]bool)
	for label := range container.Labels {
		rest, ok := strings.CutPrefix(label, prefix+".")
		if !ok {
			continue
		}
		if name, _, ok := strings.Cut(rest, "."); ok {
			names[name] = true
		}
	}
	return names
}

// configJobNames returns the names of the valid config jobs for container.
func configJobNames(container *Container, jobs []config.JobConfig) map[string]bool {
	names := make(map[string]bool)
	for _, job := range jobs {
		name := strings.TrimSpace(job.Name)
		if strings.TrimSpace(job.Container) == container.Name && validJobName.MatchString(name) {
			names[name] = true
		}
	}
	return names
}

// ConfigJobContainers returns the container names the config jobs target.
func ConfigJobContainers(jobs []config.JobConfig) map[string][]string {
	result := make(map[string][]string)
	for _, job := range jobs {
		container := strings.TrimSpace(job.Container)
		if container != "" && strings.TrimSpace(job.Name) != "" {
			result[container] = append(result[container], strings.TrimSpace(job.Name))
		}
	}
	return result
}

// LogConfigJobs logs the problems of the config file jobs and which
// containers they wait for. Their schedules are checked when the container
// starts, like labels.
func LogConfigJobs(jobs []config.JobConfig) {
	for _, problem := range CheckConfigJobs(jobs) {
		slog.Error("Invalid cron job in config file, job ignored", "problem", problem)
	}
	for container, names := range ConfigJobContainers(jobs) {
		slog.Info("Cron jobs from config file", "container", container, "jobs", strings.Join(names, ","))
	}
	if names := ConfigRunJobNames(jobs); len(names) > 0 {
		slog.Info("Cron jobs from config file that run in their own container", "jobs", strings.Join(names, ","))
	}
}

// ConfigRunJobNames returns the names of the config jobs with an image
// instead of a container.
func ConfigRunJobNames(jobs []config.JobConfig) []string {
	var names []string
	for _, job := range jobs {
		if strings.TrimSpace(job.Container) == "" && strings.TrimSpace(job.Image) != "" && strings.TrimSpace(job.Name) != "" {
			names = append(names, strings.TrimSpace(job.Name))
		}
	}
	return names
}

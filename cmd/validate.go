package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"github.com/teqneers/cronado/internal/compose"
	"github.com/teqneers/cronado/internal/config"
	cronadoCtx "github.com/teqneers/cronado/internal/context"
	"github.com/teqneers/cronado/internal/domain"
)

var validateFiles []string

// validateCmd checks cron job labels of Compose services or running
// containers with the same rules the scheduler uses.
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Check cron job labels of Compose files or running containers",
	Long: `Checks cron job labels with the same rules the scheduler uses and reports
every problem: invalid schedules, unknown properties, missing labels and more.

With -f, the labels of every service in the Compose file(s) are checked, so
mistakes are found before deployment. No Docker access is needed. Several -f
flags are merged like "docker compose -f a.yml -f b.yml".

Without -f, the labels of all running containers are checked via the Docker
socket, together with the jobs in the config file. Config jobs whose
container is not running are checked on their own.

Exits with 1 if any problem was found, 0 otherwise.`,
	Example: `  cronado validate -f docker-compose.yml
  cronado validate -f compose.yml -f compose.prod.yml
  cronado validate`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := cronadoCtx.AppCtx.Config
		var (
			targets    []validateTarget
			configJobs []config.JobConfig
			err        error
		)
		if len(validateFiles) > 0 {
			targets, err = composeTargets(validateFiles)
		} else {
			configJobs = cfg.Jobs
			targets, err = runningContainerTargets()
			targets = addConfigJobTargets(targets, configJobs)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(2)
		}

		if !validateTargets(os.Stdout, targets, configJobs, cfg.CronLabelPrefix, cfg.MinScheduleInterval, cfg.MaxTimeout) {
			os.Exit(1)
		}
	},
}

// validateTarget is a container, or a Compose service that will become one.
type validateTarget struct {
	name      string
	container *domain.Container
}

func composeTargets(files []string) ([]validateTarget, error) {
	services, err := compose.Load(files...)
	if err != nil {
		return nil, err
	}
	targets := make([]validateTarget, 0, len(services))
	for _, s := range services {
		name := s.Name
		if s.ContainerName != "" {
			name = fmt.Sprintf("%s (container %s)", s.Name, s.ContainerName)
		}
		targets = append(targets, validateTarget{
			name:      name,
			container: &domain.Container{Name: s.Name, Labels: s.Labels},
		})
	}
	return targets, nil
}

func runningContainerTargets() ([]validateTarget, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("creating Docker client: %w", err)
	}
	defer func() { _ = cli.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	containers, err := cli.ContainerList(ctx, dockercontainer.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing containers (use -f to check Compose files without Docker access): %w", err)
	}

	targets := make([]validateTarget, 0, len(containers))
	for _, c := range containers {
		name := c.ID
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		targets = append(targets, validateTarget{
			name:      name,
			container: &domain.Container{ID: c.ID, Name: name, Labels: c.Labels, Image: c.Image},
		})
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].name < targets[j].name })
	return targets, nil
}

// addConfigJobTargets adds a target for every container that config jobs
// refer to but that is not running, so that those jobs are checked too.
func addConfigJobTargets(targets []validateTarget, configJobs []config.JobConfig) []validateTarget {
	running := make(map[string]bool, len(targets))
	for _, t := range targets {
		running[t.container.Name] = true
	}
	var missing []string
	for name := range domain.ConfigJobContainers(configJobs) {
		if !running[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	for _, name := range missing {
		targets = append(targets, validateTarget{
			name:      name + " (not running)",
			container: &domain.Container{Name: name},
		})
	}
	return targets
}

// validateTargets writes a report for every target with cron job labels or
// config jobs and returns true if no problems were found.
func validateTargets(w io.Writer, targets []validateTarget, configJobs []config.JobConfig, prefix string, minScheduleInterval, maxTimeout time.Duration) bool {
	var checked, jobsOK, jobsRejected, labelsIgnored int

	configProblems := domain.CheckConfigJobs(configJobs)
	if len(configProblems) > 0 {
		_, _ = fmt.Fprintf(w, "config file\n")
		for _, problem := range configProblems {
			_, _ = fmt.Fprintf(w, "  ✗ %s\n", problem)
		}
	}
	configContainers := domain.ConfigJobContainers(configJobs)

	for _, t := range targets {
		if !hasPrefixedLabel(t.container.Labels, prefix) && len(configContainers[t.container.Name]) == 0 {
			continue
		}
		checked++
		jobs, issues := domain.ParseContainerJobs(t.container, prefix, configJobs, minScheduleInterval, maxTimeout)

		_, _ = fmt.Fprintf(w, "%s\n", t.name)

		names := make([]string, 0, len(jobs))
		for name := range jobs {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			job := jobs[name]
			state := "enabled"
			if !job.Enabled {
				state = "disabled"
			}
			if job.Source == domain.SourceConfig {
				state += ", config file"
			}
			_, _ = fmt.Fprintf(w, "  ✓ %s: %s (%s)\n", name, job.Schedule, state)
			jobsOK++
		}

		rejected := make(map[string]bool)
		for _, issue := range issues {
			if issue.JobRejected {
				if !rejected[issue.Job] {
					rejected[issue.Job] = true
					jobsRejected++
				}
				source := ""
				if issue.Source == domain.SourceConfig {
					source = " (config file)"
				}
				_, _ = fmt.Fprintf(w, "  ✗ %s: %s%s\n", issue.Job, issue.Message, source)
				continue
			}
			labelsIgnored++
			_, _ = fmt.Fprintf(w, "  ! %s: %s (label ignored)\n", issue.Label, issue.Message)
		}
	}

	if checked == 0 {
		_, _ = fmt.Fprintf(w, "No containers with %s.* labels found.\n", prefix)
		return len(configProblems) == 0
	}

	_, _ = fmt.Fprintf(w, "\nChecked %d %s: %d %s OK, %d %s rejected, %d %s ignored.\n",
		checked, plural(checked, "container", "containers"),
		jobsOK, plural(jobsOK, "job", "jobs"),
		jobsRejected, plural(jobsRejected, "job", "jobs"),
		labelsIgnored, plural(labelsIgnored, "label", "labels"))

	return jobsRejected == 0 && labelsIgnored == 0 && len(configProblems) == 0
}

func hasPrefixedLabel(labels map[string]string, prefix string) bool {
	for label := range labels {
		if strings.HasPrefix(label, prefix+".") {
			return true
		}
	}
	return false
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func init() {
	validateCmd.Flags().StringArrayVarP(&validateFiles, "file", "f", nil, "Compose file to check (repeatable)")
	rootCmd.AddCommand(validateCmd)
}

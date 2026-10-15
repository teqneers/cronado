package domain

import (
	"github.com/teqneers/cronado/internal/context"
)

type CronListItem struct {
	ContainerID   string  `json:"container_id"`
	ContainerName string  `json:"container_name,omitempty"`
	CronJob       CronJob `json:"cron_job"`
}

// jobController is implemented by managers that can trigger and pause jobs.
type jobController interface {
	RunNow(jobID string) error
	SetPaused(jobID string, paused bool) error
}

// RunJob starts the job with the given ID immediately.
func RunJob(jobID string) error {
	if m, ok := context.AppCtx.CronJobManager.(jobController); ok {
		return m.RunNow(jobID)
	}
	return ErrJobNotFound
}

// PauseJob pauses or resumes the scheduled runs of the job with the given ID.
func PauseJob(jobID string, paused bool) error {
	if m, ok := context.AppCtx.CronJobManager.(jobController); ok {
		return m.SetPaused(jobID, paused)
	}
	return ErrJobNotFound
}

// GetCronJob returns the list entry of the job with the given ID.
func GetCronJob(jobID string) (CronListItem, bool) {
	for _, item := range GetCronJobs() {
		if item.CronJob.ID == jobID {
			return item, true
		}
	}
	return CronListItem{}, false
}

// GetLabelIssues returns the label problems of all running containers.
func GetLabelIssues() []ContainerLabelIssue {
	if m, ok := context.AppCtx.CronJobManager.(interface {
		GetLabelIssues() []ContainerLabelIssue
	}); ok {
		return m.GetLabelIssues()
	}
	return []ContainerLabelIssue{}
}

// GetCronJobs returns a list of all cron jobs with their associated container info
func GetCronJobs() []CronListItem {
	allJobs := context.AppCtx.CronJobManager.GetAll()
	if len(allJobs) == 0 {
		return []CronListItem{}
	}

	cronList := []CronListItem{}
	for _, job := range allJobs {
		cronJob := job.(CronJob)
		containerName := ""
		if cronJob.Container != nil {
			containerName = cronJob.Container.Name
		}
		containerID := ""
		if cronJob.Container != nil {
			containerID = cronJob.Container.ID
		}
		cronList = append(cronList, CronListItem{
			ContainerID:   containerID,
			ContainerName: containerName,
			CronJob:       cronJob,
		})
	}
	return cronList
}

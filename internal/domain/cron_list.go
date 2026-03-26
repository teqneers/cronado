package domain

import (
	"github.com/teqneers/cronado/internal/context"
)

type CronListItem struct {
	ContainerID   string  `json:"container_id"`
	ContainerName string  `json:"container_name,omitempty"`
	CronJob       CronJob `json:"cron_job"`
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

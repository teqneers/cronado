package api

import (
	"errors"
	"net/http"

	"github.com/teqneers/cronado/internal/context"
	"github.com/teqneers/cronado/internal/domain"

	"github.com/gin-gonic/gin"
)

// RedactedCommand replaces job commands in API responses when no API token is
// configured, because commands often contain credentials.
const RedactedCommand = "[redacted: set server.api_token to show commands]"

// RedactedValue replaces other secret-bearing fields, such as ping URLs.
const RedactedValue = "[redacted]"

// GetLabelIssueList returns the label problems of all running containers:
// jobs that were rejected and labels that were ignored.
func GetLabelIssueList(c *gin.Context) {
	c.JSON(200, domain.GetLabelIssues())
}

func GetCronJobList(c *gin.Context) {
	cronList := domain.GetCronJobs()
	for i := range cronList {
		redact(&cronList[i])
	}
	c.JSON(200, cronList)
}

// redact hides secret-bearing fields when no API token is configured.
func redact(item *domain.CronListItem) {
	if context.AppCtx.Config.ServerConfig.APIToken != "" {
		return
	}
	item.CronJob.Command = RedactedCommand
	if item.CronJob.PingURL != "" {
		item.CronJob.PingURL = RedactedValue
	}
}

// RunCronJob starts a job immediately. It answers 202 once the run has
// started; the result shows up in the job list when the command finished.
func RunCronJob(c *gin.Context) {
	jobID := c.Param("id")
	if err := domain.RunJob(jobID); err != nil {
		respondJobError(c, err)
		return
	}
	respondJob(c, http.StatusAccepted, jobID)
}

// PauseCronJob stops the scheduled runs of a job until it is resumed.
func PauseCronJob(c *gin.Context) {
	setPaused(c, true)
}

// ResumeCronJob restarts the scheduled runs of a paused job.
func ResumeCronJob(c *gin.Context) {
	setPaused(c, false)
}

func setPaused(c *gin.Context, paused bool) {
	jobID := c.Param("id")
	if err := domain.PauseJob(jobID, paused); err != nil {
		respondJobError(c, err)
		return
	}
	respondJob(c, http.StatusOK, jobID)
}

func respondJob(c *gin.Context, status int, jobID string) {
	item, ok := domain.GetCronJob(jobID)
	if !ok {
		// Removed in the meantime, e.g. because its container stopped.
		c.Status(status)
		return
	}
	redact(&item)
	c.JSON(status, item)
}

func respondJobError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrJobNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrJobBusy):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

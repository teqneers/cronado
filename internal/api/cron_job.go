package api

import (
	"github.com/teqneers/cronado/internal/context"
	"github.com/teqneers/cronado/internal/domain"

	"github.com/gin-gonic/gin"
)

// RedactedCommand replaces job commands in API responses when no API token is
// configured, because commands often contain credentials.
const RedactedCommand = "[redacted: set server.api_token to show commands]"

// RedactedValue replaces other secret-bearing fields, such as ping URLs.
const RedactedValue = "[redacted]"

func GetCronJobList(c *gin.Context) {
	cronList := domain.GetCronJobs()
	if context.AppCtx.Config.ServerConfig.APIToken == "" {
		for i := range cronList {
			cronList[i].CronJob.Command = RedactedCommand
			if cronList[i].CronJob.PingURL != "" {
				cronList[i].CronJob.PingURL = RedactedValue
			}
		}
	}
	c.JSON(200, cronList)
}

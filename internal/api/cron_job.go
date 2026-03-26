package api

import (
	"github.com/teqneers/cronado/internal/domain"

	"github.com/gin-gonic/gin"
)

func GetCronJobList(c *gin.Context) {
	cronList := domain.GetCronJobs()
	c.JSON(200, cronList)
}

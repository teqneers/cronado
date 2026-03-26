package api

import (
	"github.com/teqneers/cronado/internal/context"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func SetupRouter() *gin.Engine {
	config := context.AppCtx.Config.Metrics

	r := gin.Default()

	if config.Enabled {
		// Expose Prometheus metrics endpoint
		r.GET(config.Endpoint, gin.WrapH(promhttp.Handler()))
	}

	api := r.Group("/api")
	api.GET("/cron-job", GetCronJobList)

	return r
}

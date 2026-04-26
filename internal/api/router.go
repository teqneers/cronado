package api

import (
	"github.com/teqneers/cronado/internal/context"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func SetupRouter() *gin.Engine {
	config := context.AppCtx.Config.Metrics
	apiToken := context.AppCtx.Config.ServerConfig.APIToken

	r := gin.Default()

	authMiddleware := BearerAuthMiddleware(apiToken)

	if config.Enabled {
		// Expose Prometheus metrics endpoint (protected by auth if token is set)
		r.GET(config.Endpoint, authMiddleware, gin.WrapH(promhttp.Handler()))
	}

	api := r.Group("/api", authMiddleware)
	api.GET("/cron-job", GetCronJobList)

	return r
}

package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/teqneers/cronado/internal/domain"
)

// GetHealth returns 200 when cronado is connected to Docker, 503 otherwise.
func GetHealth(c *gin.Context) {
	status, healthy := domain.CheckHealth(c.Request.Context())
	code := http.StatusOK
	if !healthy {
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, status)
}

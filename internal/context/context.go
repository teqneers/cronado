package context

import (
	"github.com/teqneers/cronado/internal/config"
)

// Forward declare the CronJobManager from domain package
// This avoids circular imports while allowing direct usage
type CronJobManager interface {
	Add(container interface{}, job interface{}) error
	Remove(containerID string)
	RemoveJob(job interface{})
	IsRegistered(jobID string) bool
	GetAll() []interface{}
	Shutdown()
}

// AppContext holds shared resources for commands
type AppContext struct {
	Config         *config.Config
	CronJobManager CronJobManager
}

// Global instance
var AppCtx *AppContext

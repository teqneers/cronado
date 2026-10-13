package domain

import (
	"context"
	"sync/atomic"
	"time"
)

// eventStreamConnected is true while cronado is subscribed to Docker events.
var eventStreamConnected atomic.Bool

// HealthStatus describes whether cronado can currently do its job.
type HealthStatus struct {
	Status      string `json:"status"` // "ok" or "unhealthy"
	Docker      string `json:"docker"` // "ok" or "unreachable"
	EventStream string `json:"event_stream"`
	Jobs        int    `json:"jobs"`
}

// CheckHealth reports healthy when the Docker daemon answers and the event
// stream is connected, which also means the initial container scan is done.
func CheckHealth(ctx context.Context) (HealthStatus, bool) {
	status := HealthStatus{Status: "ok", Docker: "ok", EventStream: "connected"}

	if !IsDockerClientInitialized() {
		status.Docker = "unreachable"
	} else {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if _, err := GetDockerClient().Ping(pingCtx); err != nil {
			status.Docker = "unreachable"
		}
	}
	if !eventStreamConnected.Load() {
		status.EventStream = "disconnected"
	}
	if manager := GetCronJobManager(); manager != nil {
		status.Jobs = manager.GetJobCount()
	}

	healthy := status.Docker == "ok" && status.EventStream == "connected"
	if !healthy {
		status.Status = "unhealthy"
	}
	return status, healthy
}

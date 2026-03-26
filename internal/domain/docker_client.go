package domain

import (
	"context"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
	"github.com/teqneers/cronado/internal/util"
)

// DockerClient defines the interface for Docker operations
type DockerClient interface {
	// Container operations
	ContainerList(ctx context.Context, options dockercontainer.ListOptions) ([]dockercontainer.Summary, error)
	ContainerInspect(ctx context.Context, containerID string) (dockercontainer.InspectResponse, error)
	ContainerExecCreate(ctx context.Context, containerID string, config dockercontainer.ExecOptions) (dockercontainer.ExecCreateResponse, error)
	ContainerExecAttach(ctx context.Context, execID string, config dockercontainer.ExecAttachOptions) (types.HijackedResponse, error)
	ContainerExecInspect(ctx context.Context, execID string) (dockercontainer.ExecInspect, error)

	// Event operations
	Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error)

	// Health check
	Ping(ctx context.Context) (types.Ping, error)
	Close() error
}

var (
	dockerClient *client.Client
)

// SetupDockerClient initializes the global Docker client
func SetupDockerClient() {
	if dockerClient == nil {
		dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			slog.Error("Failed to create Docker client", "error", err)
			util.HandleDockerClientError(err)
		}
		dockerClient = dockerCli
	}
}

// GetDockerClient returns the initialized Docker client
func GetDockerClient() *client.Client {
	return dockerClient
}

// MonitorDockerAPI continuously monitors the Docker daemon health
func MonitorDockerAPI(checkInterval time.Duration) {
	for {
		if err := pingDockerDaemon(); err != nil {
			util.HandleDockerDaemonPanic(err)
		}
		time.Sleep(checkInterval)
	}
}

// pingDockerDaemon checks if the Docker daemon is responsive
func pingDockerDaemon() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := dockerClient.Ping(ctx)
	return err
}

// IsDockerClientInitialized checks if the Docker client has been initialized
func IsDockerClientInitialized() bool {
	return dockerClient != nil
}

// CloseDockerClient closes the Docker client connection
func CloseDockerClient() error {
	if dockerClient != nil {
		return dockerClient.Close()
	}
	return nil
}

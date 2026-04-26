package domain

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/containerd/errdefs"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// CommandExecutor defines the interface for executing commands in containers
type CommandExecutor interface {
	ExecuteCommand(container *Container, jobName, command, user string, timeout time.Duration) (stdout, stderr string, err error)
}

// DockerCommandExecutor implements CommandExecutor using Docker API
type DockerCommandExecutor struct {
	client DockerClient
}

// NewDockerCommandExecutor creates a new DockerCommandExecutor
func NewDockerCommandExecutor(client DockerClient) *DockerCommandExecutor {
	return &DockerCommandExecutor{
		client: client,
	}
}

// ExecuteCommand executes a command in a container and returns its output.
func (e *DockerCommandExecutor) ExecuteCommand(container *Container, jobName, command, user string, timeout time.Duration) (string, string, error) {
	if user == "" {
		slog.Warn("No user specified, defaulting to 'root'", "container", container.DisplayName(), "command", command)
		user = "root"
	}

	execConfig := dockercontainer.ExecOptions{
		Cmd:          []string{"sh", "-c", command},
		AttachStdout: true,
		AttachStderr: true,
		User:         user,
		Tty:          false,
	}

	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	execID, err := e.client.ContainerExecCreate(ctx, container.ID, execConfig)
	switch {
	case errdefs.IsNotFound(err):
		return "", "", fmt.Errorf("container not found: %w", err)
	case errdefs.IsUnavailable(err):
		return "", "", fmt.Errorf("docker daemon unavailable: %w", err)
	case err != nil:
		return "", "", fmt.Errorf("failed to create exec instance: %w", err)
	}

	resp, err := e.client.ContainerExecAttach(ctx, execID.ID, dockercontainer.ExecAttachOptions{})
	if err != nil {
		return "", "", fmt.Errorf("failed to attach to exec instance: %w", err)
	}
	defer resp.Close()

	// Read the output into local buffers (thread-safe)
	var stdoutBuf, stderrBuf bytes.Buffer
	_, err = stdcopy.StdCopy(&stdoutBuf, &stderrBuf, resp.Reader)
	if err != nil {
		return "", "", fmt.Errorf("failed to read exec output: %w", err)
	}

	// Check the exit code
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	inspect, err := e.client.ContainerExecInspect(ctx2, execID.ID)
	if err != nil {
		return stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("failed to inspect exec instance: %w", err)
	}

	if inspect.ExitCode != 0 {
		return stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("command exited with code %d", inspect.ExitCode)
	}

	return stdoutBuf.String(), stderrBuf.String(), nil
}

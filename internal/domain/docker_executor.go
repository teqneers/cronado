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
	ExecuteCommand(container *Container, jobName, command, user string, timeout time.Duration) error
	GetOutput() (stdout, stderr string)
}

// DockerCommandExecutor implements CommandExecutor using Docker API
type DockerCommandExecutor struct {
	client DockerClient
	stdout bytes.Buffer
	stderr bytes.Buffer
}

// NewDockerCommandExecutor creates a new DockerCommandExecutor
func NewDockerCommandExecutor(client DockerClient) *DockerCommandExecutor {
	return &DockerCommandExecutor{
		client: client,
	}
}

// ExecuteCommand executes a command in a container
func (e *DockerCommandExecutor) ExecuteCommand(container *Container, jobName, command, user string, timeout time.Duration) error {
	// Reset buffers
	e.stdout.Reset()
	e.stderr.Reset()

	if user == "" {
		slog.Info("No user specified, defaulting to 'root'", "container", container.DisplayName(), "command", command)
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
	if errdefs.IsNotFound(err) {
		return fmt.Errorf("container not found: %w", err)
	} else if errdefs.IsUnavailable(err) {
		return fmt.Errorf("docker daemon unavailable: %w", err)
	} else if err != nil {
		return fmt.Errorf("failed to create exec instance: %w", err)
	}

	resp, err := e.client.ContainerExecAttach(ctx, execID.ID, dockercontainer.ExecAttachOptions{})
	if err != nil {
		return fmt.Errorf("failed to attach to exec instance: %w", err)
	}
	defer resp.Close()

	// Read the output
	_, err = stdcopy.StdCopy(&e.stdout, &e.stderr, resp.Reader)
	if err != nil {
		return fmt.Errorf("failed to read exec output: %w", err)
	}

	// Check the exit code
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	inspect, err := e.client.ContainerExecInspect(ctx2, execID.ID)
	if err != nil {
		return fmt.Errorf("failed to inspect exec instance: %w", err)
	}

	if inspect.ExitCode != 0 {
		return fmt.Errorf("command exited with code %d", inspect.ExitCode)
	}

	return nil
}

// GetOutput returns the stdout and stderr from the last command execution
func (e *DockerCommandExecutor) GetOutput() (stdout, stderr string) {
	return e.stdout.String(), e.stderr.String()
}
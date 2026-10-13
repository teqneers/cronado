package domain

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strconv"
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

// maxOutputBytes caps how much stdout/stderr is kept per execution. Only the
// tail is kept, since the end of the output usually explains a failure.
const maxOutputBytes = 64 * 1024

// killGrace is the extra time given to the in-container `timeout` wrapper to
// kill the command and close the stream before cronado gives up waiting.
const killGrace = 10 * time.Second

// timeoutWrapper runs the command under `timeout -s KILL` when the container
// provides it (busybox and coreutils both do), so a timed-out command is
// actually stopped instead of running on after cronado stops listening.
// $1 is the timeout in seconds, $2 the command.
const timeoutWrapper = `if command -v timeout >/dev/null 2>&1; then exec timeout -s KILL "$1" sh -c "$2"; fi; exec sh -c "$2"`

// exitCodeKilled is the exit code of a command killed by SIGKILL.
const exitCodeKilled = 137

// ExecuteCommand executes a command in a container and returns its output.
// An empty user runs the command as the container's configured user.
func (e *DockerCommandExecutor) ExecuteCommand(container *Container, jobName, command, user string, timeout time.Duration) (string, string, error) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	timeoutSecs := int(math.Ceil(timeout.Seconds()))

	execConfig := dockercontainer.ExecOptions{
		Cmd:          []string{"sh", "-c", timeoutWrapper, "cronado", strconv.Itoa(timeoutSecs), command},
		AttachStdout: true,
		AttachStderr: true,
		User:         user,
		Tty:          false,
	}

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout+killGrace)
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

	// Read the output into local, size-capped buffers (thread-safe)
	stdoutBuf := newTailBuffer(maxOutputBytes)
	stderrBuf := newTailBuffer(maxOutputBytes)
	_, err = stdcopy.StdCopy(stdoutBuf, stderrBuf, resp.Reader)
	if err != nil {
		if ctx.Err() != nil {
			return stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("command timed out after %s and could not be stopped (no `timeout` binary in container?)", timeout)
		}
		return stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("failed to read exec output: %w", err)
	}

	// Check the exit code
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	inspect, err := e.client.ContainerExecInspect(ctx2, execID.ID)
	if err != nil {
		return stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("failed to inspect exec instance: %w", err)
	}

	if inspect.ExitCode == exitCodeKilled && time.Since(startTime) >= timeout {
		return stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("command timed out after %s and was killed", timeout)
	}
	if inspect.ExitCode != 0 {
		return stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("command exited with code %d", inspect.ExitCode)
	}

	return stdoutBuf.String(), stderrBuf.String(), nil
}

// tailBuffer is an io.Writer that keeps only the last limit bytes written.
type tailBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func newTailBuffer(limit int) *tailBuffer {
	return &tailBuffer{limit: limit}
}

// Write never fails, so the stream is always drained even when output is dropped.
func (t *tailBuffer) Write(p []byte) (int, error) {
	t.buf.Write(p)
	if over := t.buf.Len() - t.limit; over > 0 {
		t.buf.Next(over)
		t.truncated = true
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	if t.truncated {
		return "[output truncated]\n" + t.buf.String()
	}
	return t.buf.String()
}

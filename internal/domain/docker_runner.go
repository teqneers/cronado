package domain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ContainerRunner runs jobs with an image in a new container.
type ContainerRunner interface {
	RunContainer(job *CronJob) (stdout, stderr string, err error)
}

// ContainerRunClient defines the Docker operations needed to run a job in
// its own container.
type ContainerRunClient interface {
	ContainerList(ctx context.Context, options dockercontainer.ListOptions) ([]dockercontainer.Summary, error)
	ImageInspect(ctx context.Context, imageID string, inspectOpts ...client.ImageInspectOption) (image.InspectResponse, error)
	ImagePull(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error)
	ContainerCreate(ctx context.Context, config *dockercontainer.Config, hostConfig *dockercontainer.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (dockercontainer.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options dockercontainer.StartOptions) error
	ContainerWait(ctx context.Context, containerID string, condition dockercontainer.WaitCondition) (<-chan dockercontainer.WaitResponse, <-chan error)
	ContainerLogs(ctx context.Context, containerID string, options dockercontainer.LogsOptions) (io.ReadCloser, error)
	ContainerKill(ctx context.Context, containerID, signal string) error
	ContainerRemove(ctx context.Context, containerID string, options dockercontainer.RemoveOptions) error
}

// RunJobLabel marks the containers cronado creates for jobs with an image.
// Its value is the job ID. It is outside the cron label prefix so that these
// containers are never read as job definitions.
const RunJobLabel = "io.github.teqneers.cronado.job"

// pullTimeout limits how long pulling a missing image may take. It is not
// part of the job timeout, so a first run with a large image does not fail.
const pullTimeout = 10 * time.Minute

// DockerContainerRunner implements ContainerRunner using the Docker API
type DockerContainerRunner struct {
	client ContainerRunClient
}

// NewDockerContainerRunner creates a new DockerContainerRunner
func NewDockerContainerRunner(client ContainerRunClient) *DockerContainerRunner {
	return &DockerContainerRunner{client: client}
}

// RunContainer pulls the job's image if missing, runs the command in a new
// container until it exits or the timeout kills it, and removes the
// container. The command is passed to the image's entrypoint as arguments,
// like "docker run image cmd"; an empty command runs the image's default.
func (r *DockerContainerRunner) RunContainer(job *CronJob) (string, string, error) {
	timeout := job.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	args, err := splitArgs(job.Command)
	if err != nil {
		return "", "", err
	}

	if err := r.ensureImage(job.Image); err != nil {
		return "", "", err
	}

	createCtx, cancelCreate := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelCreate()
	created, err := r.client.ContainerCreate(createCtx,
		&dockercontainer.Config{
			Image:  job.Image,
			Cmd:    args,
			User:   job.User,
			Env:    job.Env,
			Labels: map[string]string{RunJobLabel: job.ID},
		},
		&dockercontainer.HostConfig{
			Binds:       job.Volumes,
			NetworkMode: dockercontainer.NetworkMode(job.Network),
		},
		nil, nil, fmt.Sprintf("cronado-%s-%d", job.ID, time.Now().Unix()))
	if err != nil {
		return "", "", fmt.Errorf("failed to create container from image %s: %w", job.Image, err)
	}
	defer r.remove(created.ID)

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := r.client.ContainerStart(ctx, created.ID, dockercontainer.StartOptions{}); err != nil {
		return "", "", fmt.Errorf("failed to start container from image %s: %w", job.Image, err)
	}

	exitCode, runErr := r.wait(ctx, created.ID)
	if runErr != nil && ctx.Err() != nil {
		r.kill(created.ID)
		runErr = fmt.Errorf("command timed out after %s and was killed", timeout)
	}
	stdout, stderr := r.logs(created.ID)
	if runErr != nil {
		return stdout, stderr, runErr
	}
	if exitCode == exitCodeKilled && time.Since(startTime) >= timeout {
		return stdout, stderr, fmt.Errorf("command timed out after %s and was killed", timeout)
	}
	if exitCode != 0 {
		return stdout, stderr, fmt.Errorf("command exited with code %d", exitCode)
	}
	return stdout, stderr, nil
}

// ensureImage pulls the image if it is not present locally.
func (r *DockerContainerRunner) ensureImage(ref string) error {
	ctx, cancel := context.WithTimeout(context.Background(), pullTimeout)
	defer cancel()

	_, err := r.client.ImageInspect(ctx, ref)
	if err == nil {
		return nil
	}
	if !errdefs.IsNotFound(err) {
		return fmt.Errorf("failed to inspect image %s: %w", ref, err)
	}

	slog.Info("Pulling image for cron job", "image", ref)
	reader, err := r.client.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", ref, err)
	}
	defer func() { _ = reader.Close() }()
	// The pull runs until its progress stream is read to the end.
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", ref, err)
	}
	return nil
}

// wait blocks until the container exits and returns its exit code.
func (r *DockerContainerRunner) wait(ctx context.Context, containerID string) (int64, error) {
	resultChan, errChan := r.client.ContainerWait(ctx, containerID, dockercontainer.WaitConditionNotRunning)
	select {
	case result := <-resultChan:
		if result.Error != nil && result.Error.Message != "" {
			return 0, fmt.Errorf("failed to wait for container: %s", result.Error.Message)
		}
		return result.StatusCode, nil
	case err := <-errChan:
		return 0, fmt.Errorf("failed to wait for container: %w", err)
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// logs returns the tail of the container's stdout and stderr. A log driver
// that cannot be read back only costs the output, not the result.
func (r *DockerContainerRunner) logs(containerID string) (string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stdoutBuf := newTailBuffer(maxOutputBytes)
	stderrBuf := newTailBuffer(maxOutputBytes)
	reader, err := r.client.ContainerLogs(ctx, containerID, dockercontainer.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		slog.Warn("Failed to read output of job container", "container", shortID(containerID), "error", err)
		return "", ""
	}
	defer func() { _ = reader.Close() }()
	if _, err := stdcopy.StdCopy(stdoutBuf, stderrBuf, reader); err != nil {
		slog.Warn("Failed to read output of job container", "container", shortID(containerID), "error", err)
	}
	return stdoutBuf.String(), stderrBuf.String()
}

// kill stops a container whose job timed out.
func (r *DockerContainerRunner) kill(containerID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := r.client.ContainerKill(ctx, containerID, "KILL"); err != nil && !errdefs.IsNotFound(err) {
		slog.Warn("Failed to kill timed-out job container", "container", shortID(containerID), "error", err)
	}
}

// remove deletes a job container, killing it first if it still runs.
func (r *DockerContainerRunner) remove(containerID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := r.client.ContainerRemove(ctx, containerID, dockercontainer.RemoveOptions{Force: true, RemoveVolumes: true}); err != nil && !errdefs.IsNotFound(err) {
		slog.Warn("Failed to remove job container", "container", shortID(containerID), "error", err)
	}
}

// RemoveLeftoverRunContainers removes stopped job containers that a previous
// cronado process could not remove, e.g. because it was killed during a run.
// Running ones are left alone; they are removed at a later start.
func RemoveLeftoverRunContainers(client ContainerRunClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	containers, err := client.ContainerList(ctx, dockercontainer.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", RunJobLabel)),
	})
	if err != nil {
		slog.Warn("Failed to list leftover job containers", "error", err)
		return
	}
	runner := NewDockerContainerRunner(client)
	for _, c := range containers {
		if c.State == dockercontainer.StateRunning {
			continue
		}
		slog.Info("Removing leftover job container", "container", shortID(c.ID), "job_id", c.Labels[RunJobLabel])
		runner.remove(c.ID)
	}
}

// IsRunContainer reports whether container was created by cronado to run a
// job with an image.
func IsRunContainer(container *Container) bool {
	return container.HasLabel(RunJobLabel)
}

// shortID returns the first 12 characters of a container ID.
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// errUnterminatedQuote is returned by splitArgs for a missing closing quote
// or a trailing backslash.
var errUnterminatedQuote = errors.New("unterminated quote or trailing backslash in command")

// splitArgs splits a command into arguments like a POSIX shell, with single
// and double quotes and backslash escapes, but without expansions.
func splitArgs(command string) ([]string, error) {
	var args []string
	var current strings.Builder
	inArg := false
	var quote rune
	escaped := false

	for _, ch := range command {
		switch {
		case escaped:
			// In double quotes, a backslash only escapes these characters.
			if quote == '"' && !strings.ContainsRune("\"\\$`", ch) {
				current.WriteRune('\\')
			}
			current.WriteRune(ch)
			escaped = false
		case quote == '\'':
			if ch == '\'' {
				quote = 0
			} else {
				current.WriteRune(ch)
			}
		case ch == '\\':
			escaped = true
			inArg = true
		case quote == '"':
			if ch == '"' {
				quote = 0
			} else {
				current.WriteRune(ch)
			}
		case ch == '\'' || ch == '"':
			quote = ch
			inArg = true
		case ch == ' ' || ch == '\t' || ch == '\n':
			if inArg {
				args = append(args, current.String())
				current.Reset()
				inArg = false
			}
		default:
			current.WriteRune(ch)
			inArg = true
		}
	}
	if quote != 0 || escaped {
		return nil, errUnterminatedQuote
	}
	if inArg {
		args = append(args, current.String())
	}
	return args, nil
}

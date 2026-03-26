package domain

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
)

// MockDockerClient implements the DockerClient interface for testing
type MockDockerClient struct {
	execCreateResp  dockercontainer.ExecCreateResponse
	execCreateErr   error
	execCreateCalls int

	execAttachResp types.HijackedResponse
	execAttachErr  error

	execInspectResult dockercontainer.ExecInspect
	execInspectErr    error

	inspectResult dockercontainer.InspectResponse
	inspectErr    error
}

func (m *MockDockerClient) ContainerList(_ context.Context, _ dockercontainer.ListOptions) ([]dockercontainer.Summary, error) {
	return nil, nil
}

func (m *MockDockerClient) ContainerInspect(_ context.Context, _ string) (dockercontainer.InspectResponse, error) {
	return m.inspectResult, m.inspectErr
}

func (m *MockDockerClient) ContainerExecCreate(_ context.Context, _ string, _ dockercontainer.ExecOptions) (dockercontainer.ExecCreateResponse, error) {
	m.execCreateCalls++
	return m.execCreateResp, m.execCreateErr
}

func (m *MockDockerClient) ContainerExecAttach(_ context.Context, _ string, _ dockercontainer.ExecAttachOptions) (types.HijackedResponse, error) {
	return m.execAttachResp, m.execAttachErr
}

func (m *MockDockerClient) ContainerExecInspect(_ context.Context, _ string) (dockercontainer.ExecInspect, error) {
	return m.execInspectResult, m.execInspectErr
}

func (m *MockDockerClient) Events(_ context.Context, _ events.ListOptions) (<-chan events.Message, <-chan error) {
	return nil, nil
}

func (m *MockDockerClient) Ping(_ context.Context) (types.Ping, error) {
	return types.Ping{}, nil
}

func (m *MockDockerClient) Close() error {
	return nil
}

func TestDockerCommandExecutor_ExecCreateError(t *testing.T) {
	container := &Container{ID: "abc123456789", Name: "test-container"}
	mockClient := &MockDockerClient{
		execCreateErr: errors.New("create failed"),
	}
	executor := NewDockerCommandExecutor(mockClient)

	err := executor.ExecuteCommand(container, "test-job", "echo hello", "root", 30*time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
	if mockClient.execCreateCalls != 1 {
		t.Errorf("expected 1 exec create call, got %d", mockClient.execCreateCalls)
	}
}

func TestDockerCommandExecutor_ExecAttachError(t *testing.T) {
	container := &Container{ID: "abc123456789", Name: "test-container"}
	mockClient := &MockDockerClient{
		execCreateResp: dockercontainer.ExecCreateResponse{ID: "exec-123"},
		execAttachErr:  errors.New("attach failed"),
	}
	executor := NewDockerCommandExecutor(mockClient)

	err := executor.ExecuteCommand(container, "test-job", "echo hello", "root", 30*time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDockerCommandExecutor_DefaultUserWhenEmpty(t *testing.T) {
	container := &Container{ID: "abc123456789", Name: "test-container"}
	mockClient := &MockDockerClient{
		execCreateResp: dockercontainer.ExecCreateResponse{ID: "exec-123"},
		execAttachErr:  errors.New("attach failed"),
	}
	executor := NewDockerCommandExecutor(mockClient)

	// Should not panic and should default to root
	err := executor.ExecuteCommand(container, "test-job", "echo hello", "", 30*time.Second)
	if err == nil {
		t.Fatal("expected error from attach")
	}
}

func TestDockerCommandExecutor_DefaultTimeoutWhenZero(t *testing.T) {
	container := &Container{ID: "abc123456789", Name: "test-container"}
	mockClient := &MockDockerClient{
		execCreateErr: errors.New("create failed"),
	}
	executor := NewDockerCommandExecutor(mockClient)

	// Should use DefaultTimeout when timeout is 0
	err := executor.ExecuteCommand(container, "test-job", "echo hello", "root", 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDockerCommandExecutor_DefaultTimeoutWhenNegative(t *testing.T) {
	container := &Container{ID: "abc123456789", Name: "test-container"}
	mockClient := &MockDockerClient{
		execCreateErr: errors.New("create failed"),
	}
	executor := NewDockerCommandExecutor(mockClient)

	err := executor.ExecuteCommand(container, "test-job", "echo hello", "root", -1*time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDockerCommandExecutor_GetOutput(t *testing.T) {
	mockClient := &MockDockerClient{}
	executor := NewDockerCommandExecutor(mockClient)

	stdout, stderr := executor.GetOutput()
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Errorf("expected empty stderr, got %q", stderr)
	}
}

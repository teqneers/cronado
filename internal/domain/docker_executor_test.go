package domain

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/containerd/errdefs"
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

// mockConn implements net.Conn for testing HijackedResponse
type mockConn struct {
	io.Reader
}

func (m *mockConn) Write(b []byte) (int, error)        { return len(b), nil }
func (m *mockConn) Close() error                       { return nil }
func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(_ time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(_ time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(_ time.Time) error { return nil }

// createDockerStreamData builds Docker multiplexed stream data.
// streamType: 1 = stdout, 2 = stderr
func createDockerStreamData(streamType byte, message string) []byte {
	data := []byte(message)
	header := make([]byte, 8)
	header[0] = streamType
	binary.BigEndian.PutUint32(header[4:], uint32(len(data)))
	return append(header, data...)
}

func createMockHijackedResponse(data []byte) types.HijackedResponse {
	conn := &mockConn{Reader: bytes.NewReader(data)}
	return types.NewHijackedResponse(conn, "application/octet-stream")
}

func TestDockerCommandExecutor_SuccessfulExecution(t *testing.T) {
	container := &Container{ID: "abc123456789", Name: "test-container"}

	stdoutData := createDockerStreamData(1, "hello world")
	mockClient := &MockDockerClient{
		execCreateResp: dockercontainer.ExecCreateResponse{ID: "exec-123"},
		execAttachResp: createMockHijackedResponse(stdoutData),
		execInspectResult: dockercontainer.ExecInspect{
			ExitCode: 0,
		},
	}
	executor := NewDockerCommandExecutor(mockClient)

	stdout, stderr, err := executor.ExecuteCommand(container, "test-job", "echo hello", "root", 30*time.Second)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stdout != "hello world" {
		t.Errorf("expected stdout 'hello world', got %q", stdout)
	}
	if stderr != "" {
		t.Errorf("expected empty stderr, got %q", stderr)
	}
}

func TestDockerCommandExecutor_StdoutAndStderr(t *testing.T) {
	container := &Container{ID: "abc123456789", Name: "test-container"}

	data := append(
		createDockerStreamData(1, "output"),
		createDockerStreamData(2, "warning")...,
	)
	mockClient := &MockDockerClient{
		execCreateResp:    dockercontainer.ExecCreateResponse{ID: "exec-123"},
		execAttachResp:    createMockHijackedResponse(data),
		execInspectResult: dockercontainer.ExecInspect{ExitCode: 0},
	}
	executor := NewDockerCommandExecutor(mockClient)

	stdout, stderr, err := executor.ExecuteCommand(container, "test-job", "cmd", "root", 30*time.Second)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stdout != "output" {
		t.Errorf("expected stdout 'output', got %q", stdout)
	}
	if stderr != "warning" {
		t.Errorf("expected stderr 'warning', got %q", stderr)
	}
}

func TestDockerCommandExecutor_NonZeroExitCode(t *testing.T) {
	container := &Container{ID: "abc123456789", Name: "test-container"}

	data := append(
		createDockerStreamData(1, "some output"),
		createDockerStreamData(2, "error message")...,
	)
	mockClient := &MockDockerClient{
		execCreateResp:    dockercontainer.ExecCreateResponse{ID: "exec-123"},
		execAttachResp:    createMockHijackedResponse(data),
		execInspectResult: dockercontainer.ExecInspect{ExitCode: 127},
	}
	executor := NewDockerCommandExecutor(mockClient)

	stdout, stderr, err := executor.ExecuteCommand(container, "test-job", "missing-cmd", "root", 30*time.Second)
	if err == nil {
		t.Fatal("expected error for non-zero exit code")
	}
	if !strings.Contains(err.Error(), "command exited with code 127") {
		t.Errorf("expected exit code 127 error, got %v", err)
	}
	if stdout != "some output" {
		t.Errorf("expected stdout 'some output', got %q", stdout)
	}
	if stderr != "error message" {
		t.Errorf("expected stderr 'error message', got %q", stderr)
	}
}

func TestDockerCommandExecutor_ExecInspectError(t *testing.T) {
	container := &Container{ID: "abc123456789", Name: "test-container"}

	data := createDockerStreamData(1, "partial output")
	mockClient := &MockDockerClient{
		execCreateResp: dockercontainer.ExecCreateResponse{ID: "exec-123"},
		execAttachResp: createMockHijackedResponse(data),
		execInspectErr: errors.New("inspect failed"),
	}
	executor := NewDockerCommandExecutor(mockClient)

	stdout, _, err := executor.ExecuteCommand(container, "test-job", "echo hello", "root", 30*time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to inspect exec instance") {
		t.Errorf("expected inspect error, got %v", err)
	}
	// Output should still be returned even on inspect error
	if stdout != "partial output" {
		t.Errorf("expected stdout 'partial output', got %q", stdout)
	}
}

func TestDockerCommandExecutor_ContainerNotFound(t *testing.T) {
	container := &Container{ID: "nonexistent", Name: "missing-container"}
	mockClient := &MockDockerClient{
		execCreateErr: errdefs.ErrNotFound,
	}
	executor := NewDockerCommandExecutor(mockClient)

	_, _, err := executor.ExecuteCommand(container, "test-job", "echo hello", "root", 30*time.Second)
	if err == nil {
		t.Fatal("expected error for missing container")
	}
	if !strings.Contains(err.Error(), "container not found") {
		t.Errorf("expected 'container not found' error, got %v", err)
	}
}

func TestDockerCommandExecutor_DockerDaemonUnavailable(t *testing.T) {
	container := &Container{ID: "abc123456789", Name: "test-container"}
	mockClient := &MockDockerClient{
		execCreateErr: errdefs.ErrUnavailable,
	}
	executor := NewDockerCommandExecutor(mockClient)

	_, _, err := executor.ExecuteCommand(container, "test-job", "echo hello", "root", 30*time.Second)
	if err == nil {
		t.Fatal("expected error for unavailable daemon")
	}
	if !strings.Contains(err.Error(), "docker daemon unavailable") {
		t.Errorf("expected 'docker daemon unavailable' error, got %v", err)
	}
}

func TestDockerCommandExecutor_ExecCreateError(t *testing.T) {
	container := &Container{ID: "abc123456789", Name: "test-container"}
	mockClient := &MockDockerClient{
		execCreateErr: errors.New("create failed"),
	}
	executor := NewDockerCommandExecutor(mockClient)

	_, _, err := executor.ExecuteCommand(container, "test-job", "echo hello", "root", 30*time.Second)
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

	_, _, err := executor.ExecuteCommand(container, "test-job", "echo hello", "root", 30*time.Second)
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

	_, _, err := executor.ExecuteCommand(container, "test-job", "echo hello", "", 30*time.Second)
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

	_, _, err := executor.ExecuteCommand(container, "test-job", "echo hello", "root", 0)
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

	_, _, err := executor.ExecuteCommand(container, "test-job", "echo hello", "root", -1*time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
}

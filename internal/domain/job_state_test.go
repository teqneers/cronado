package domain

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/teqneers/cronado/internal/config"
	cronadoCtx "github.com/teqneers/cronado/internal/context"
)

// runJobOnce registers job, runs it once through the scheduler and returns
// the stored job afterwards.
func runJobOnce(t *testing.T, manager *CronJobManager, scheduler *MockScheduler, container *Container, job CronJob) *CronJob {
	t.Helper()
	if err := manager.Add(container, job); err != nil {
		t.Fatalf("Failed to add job: %v", err)
	}
	stored, _ := manager.GetJobByID(job.ID)
	scheduler.ExecuteJob(stored.SchedulerId)
	stored, _ = manager.GetJobByID(job.ID)
	return stored
}

func TestExecuteJob_RecordsSuccessState(t *testing.T) {
	manager, scheduler, _, _, _ := createTestCronJobManager()
	container := createTestCronJobManagerContainer()

	before := time.Now()
	job := runJobOnce(t, manager, scheduler, container, createTestCronJob(container))

	if job.LastResult != "success" || job.LastError != "" {
		t.Errorf("LastResult = %q, LastError = %q", job.LastResult, job.LastError)
	}
	if job.LastRun == nil || job.LastRun.Before(before) {
		t.Errorf("LastRun = %v, want >= %v", job.LastRun, before)
	}
	if job.LastSuccess == nil || !job.LastSuccess.Equal(*job.LastRun) {
		t.Errorf("LastSuccess = %v, want %v", job.LastSuccess, job.LastRun)
	}
}

func TestExecuteJob_RecordsFailureStateAndOutput(t *testing.T) {
	manager, scheduler, executor, _, notifier := createTestCronJobManager()
	container := createTestCronJobManagerContainer()
	executor.SetShouldFail(true)
	executor.SetOutput("some stdout", "pg_dump: connection refused")

	job := runJobOnce(t, manager, scheduler, container, createTestCronJob(container))

	if job.LastResult != "failure" || job.LastError != "mock execution failed" {
		t.Errorf("LastResult = %q, LastError = %q", job.LastResult, job.LastError)
	}
	if job.LastSuccess != nil {
		t.Errorf("LastSuccess = %v, want nil", job.LastSuccess)
	}
	if notifier.lastOutput != "pg_dump: connection refused" {
		t.Errorf("notification output = %q, want stderr", notifier.lastOutput)
	}
}

func TestGetAllTyped_IncludesNextRun(t *testing.T) {
	manager, scheduler, _, _, _ := createTestCronJobManager()
	container := createTestCronJobManagerContainer()
	next := time.Date(2030, 1, 1, 2, 0, 0, 0, time.UTC)
	scheduler.SetNext(next)

	if err := manager.Add(container, createTestCronJob(container)); err != nil {
		t.Fatal(err)
	}
	jobs := manager.GetAllTyped()
	if len(jobs) != 1 || jobs[0].NextRun == nil || !jobs[0].NextRun.Equal(next) {
		t.Fatalf("NextRun = %v, want %v", jobs[0].NextRun, next)
	}
}

func TestExecuteJob_SendsPings(t *testing.T) {
	var mu sync.Mutex
	var paths, bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		paths = append(paths, r.URL.Path)
		bodies = append(bodies, string(body))
		mu.Unlock()
	}))
	defer server.Close()

	// httptest listens on loopback, which is blocked by default.
	originalAllow := allowPrivatePingTargets
	allowPrivatePingTargets = func() bool { return true }
	defer func() { allowPrivatePingTargets = originalAllow }()

	tests := []struct {
		name      string
		fail      bool
		wantPaths []string
	}{
		{name: "success", fail: false, wantPaths: []string{"/ping/abc/start", "/ping/abc"}},
		{name: "failure", fail: true, wantPaths: []string{"/ping/abc/start", "/ping/abc/fail"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths, bodies = nil, nil
			manager, scheduler, executor, _, _ := createTestCronJobManager()
			executor.SetShouldFail(tt.fail)
			executor.SetOutput("", "disk full")
			container := createTestCronJobManagerContainer()
			job := createTestCronJob(container)
			job.PingURL = server.URL + "/ping/abc"

			runJobOnce(t, manager, scheduler, container, job)

			mu.Lock()
			defer mu.Unlock()
			if strings.Join(paths, ",") != strings.Join(tt.wantPaths, ",") {
				t.Errorf("paths = %v, want %v", paths, tt.wantPaths)
			}
			if tt.fail && !strings.Contains(bodies[1], "disk full") {
				t.Errorf("fail ping body = %q, want output", bodies[1])
			}
		})
	}
}

func TestCronJobBuilder_SetPingURL(t *testing.T) {
	tests := []struct {
		input     string
		wantError bool
	}{
		{input: "https://hc-ping.com/5f2c1a", wantError: false},
		{input: `"http://healthchecks.local/ping/abc"`, wantError: false},
		{input: "", wantError: false},
		{input: "ftp://example.com/x", wantError: true},
		{input: "file:///etc/passwd", wantError: true},
		{input: "hc-ping.com/abc", wantError: true},
	}
	for _, tt := range tests {
		builder := NewCronJobBuilder("job", createTestContainer("test-123"), 0, 0)
		builder.SetPingURL(tt.input)
		if builder.HasErrors() != tt.wantError {
			t.Errorf("SetPingURL(%q) errors = %v, want error %v", tt.input, builder.GetErrors(), tt.wantError)
		}
	}
}

func TestOutputTail(t *testing.T) {
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "line"
	}
	lines[29] = "last"
	tail := outputTail(strings.Join(lines, "\n")+"\n\n", 20, 1000)
	if strings.Count(tail, "\n") != 19 || !strings.HasSuffix(tail, "last") {
		t.Errorf("tail = %q", tail)
	}
	if got := outputTail(strings.Repeat("x", 50), 20, 10); got != strings.Repeat("x", 10) {
		t.Errorf("byte cap: got %q", got)
	}
	if got := outputTail("  \n", 20, 10); got != "" {
		t.Errorf("blank output: got %q", got)
	}
}

func TestFailureMessage(t *testing.T) {
	original := cronadoCtx.AppCtx
	defer func() { cronadoCtx.AppCtx = original }()
	container := &Container{ID: "abc123456789", Name: "db"}

	for _, include := range []bool{true, false} {
		cronadoCtx.AppCtx = &cronadoCtx.AppContext{Config: &config.Config{Notify: config.Notify{IncludeOutput: include}}}
		title, message := failureMessage(container, "backup", errors.New("command exited with code 1"), "pg_dump: boom\n")

		if title != "Cron Job Failed: backup on db" {
			t.Errorf("title = %q", title)
		}
		if !strings.Contains(message, "command exited with code 1") {
			t.Errorf("message lacks error: %q", message)
		}
		if strings.Contains(message, "pg_dump: boom") != include {
			t.Errorf("include_output=%v: message %q", include, message)
		}
	}
}

func TestJobStateCollector(t *testing.T) {
	manager, scheduler, _, _, _ := createTestCronJobManager()
	container := createTestCronJobManagerContainer()
	scheduler.SetNext(time.Now().Add(time.Hour))
	runJobOnce(t, manager, scheduler, container, createTestCronJob(container))

	collector := NewJobStateCollector(manager)
	// last_run, last_success, next_run, running and paused for one job
	if n := testutil.CollectAndCount(collector); n != 5 {
		t.Errorf("collected %d metrics, want 5", n)
	}
	if n := testutil.CollectAndCount(collector, "cronado_job_running"); n != 1 {
		t.Errorf("collected %d running metrics, want 1", n)
	}
}

func TestCheckHealth_DisconnectedStream(t *testing.T) {
	eventStreamConnected.Store(false)
	status, healthy := CheckHealth(t.Context())
	if healthy || status.Status != "unhealthy" || status.EventStream != "disconnected" {
		t.Errorf("status = %+v, healthy = %v", status, healthy)
	}
}

func TestSendPing_BlocksInternalTargetsByDefault(t *testing.T) {
	var hits int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
	}))
	defer server.Close()

	originalAllow := allowPrivatePingTargets
	allowPrivatePingTargets = func() bool { return false }
	defer func() { allowPrivatePingTargets = originalAllow }()

	sendPing(&CronJob{ID: "job", PingURL: server.URL + "/secret-uuid"}, pingSuccess, "")

	mu.Lock()
	defer mu.Unlock()
	if hits != 0 {
		t.Errorf("loopback ping target was reached %d times, want 0", hits)
	}
}

func TestIsInternalIP(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1": true, "10.1.2.3": true, "172.17.0.1": true, "192.168.1.10": true,
		"169.254.169.254": true, "100.64.0.1": true, "0.0.0.0": true, "::1": true, "fd00::1": true,
		"198.18.0.1": true, "240.0.0.1": true, "255.255.255.255": true, "224.0.0.1": true,
		"::ffff:127.0.0.1": true, "::ffff:10.0.0.1": true, "::127.0.0.1": true,
		"64:ff9b::7f00:1": true, "2002:7f00:1::": true, "2001:0:4136:e378::1": true, "fe80::1": true,
		"8.8.8.8": false, "1.1.1.1": false, "::ffff:8.8.8.8": false, "2001:4860:4860::8888": false,
	} {
		if got := isInternalIP(net.ParseIP(addr)); got != want {
			t.Errorf("isInternalIP(%s) = %v, want %v", addr, got, want)
		}
	}
}

func TestPingErrorsDoNotContainURL(t *testing.T) {
	const secret = "5f2c1a3e-secret-uuid"
	if err := validatePingURL("ftp://hc-ping.com/" + secret); err == nil || strings.Contains(err.Error(), secret) {
		t.Errorf("validation error leaks URL: %v", err)
	}

	_, err := pingClient.Post("http://127.0.0.1:1/"+secret, "text/plain", nil)
	if err == nil {
		t.Fatal("expected connection error")
	}
	if !strings.Contains(err.Error(), secret) {
		t.Fatal("precondition: raw client error should contain the URL")
	}
	if strings.Contains(stripURL(err).Error(), secret) {
		t.Errorf("stripped error leaks URL: %v", stripURL(err))
	}
}

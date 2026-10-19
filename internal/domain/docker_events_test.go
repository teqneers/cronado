package domain

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

func TestParseCronsFromContainer_TimeoutLabel(t *testing.T) {
	container := &Container{
		ID:   "abc123456789",
		Name: "test-container",
		Labels: map[string]string{
			"cronado.backup.enabled":  "true",
			"cronado.backup.schedule": "@every 1h",
			"cronado.backup.cmd":      "pg_dump mydb",
			"cronado.backup.user":     "postgres",
			"cronado.backup.timeout":  "10m",
		},
	}

	result := parseCronsFromContainer(container, "cronado")

	if len(result) != 1 {
		t.Fatalf("expected 1 cron job, got %d", len(result))
	}

	job, ok := result["backup"]
	if !ok {
		t.Fatal("expected 'backup' job")
	}

	if job.Timeout != 10*time.Minute {
		t.Errorf("expected timeout 10m, got %v", job.Timeout)
	}
}

func TestParseCronsFromContainer_DefaultTimeout(t *testing.T) {
	container := &Container{
		ID:   "abc123456789",
		Name: "test-container",
		Labels: map[string]string{
			"cronado.backup.enabled":  "true",
			"cronado.backup.schedule": "@every 1h",
			"cronado.backup.cmd":      "echo hello",
		},
	}

	result := parseCronsFromContainer(container, "cronado")

	if len(result) != 1 {
		t.Fatalf("expected 1 cron job, got %d", len(result))
	}

	job := result["backup"]
	if job.Timeout != DefaultTimeout {
		t.Errorf("expected default timeout %v, got %v", DefaultTimeout, job.Timeout)
	}
}

func TestParseCronsFromContainer_InvalidTimeout(t *testing.T) {
	container := &Container{
		ID:   "abc123456789",
		Name: "test-container",
		Labels: map[string]string{
			"cronado.backup.enabled":  "true",
			"cronado.backup.schedule": "@every 1h",
			"cronado.backup.cmd":      "echo hello",
			"cronado.backup.timeout":  "notaduration",
		},
	}

	result := parseCronsFromContainer(container, "cronado")

	// Should fail to build because of invalid timeout
	if len(result) != 0 {
		t.Errorf("expected 0 cron jobs (invalid timeout should fail build), got %d", len(result))
	}
}

func TestParseCronsFromContainer_UnknownProperty(t *testing.T) {
	container := &Container{
		ID:   "abc123456789",
		Name: "test-container",
		Labels: map[string]string{
			"cronado.backup.enabled":  "true",
			"cronado.backup.schedule": "@every 1h",
			"cronado.backup.cmd":      "echo hello",
			"cronado.backup.unknown":  "value",
		},
	}

	// Should still parse valid job, just log a warning for unknown property
	result := parseCronsFromContainer(container, "cronado")

	if len(result) != 1 {
		t.Fatalf("expected 1 cron job, got %d", len(result))
	}
}

func TestParseCronsFromContainer_MultipleJobs(t *testing.T) {
	container := &Container{
		ID:   "abc123456789",
		Name: "test-container",
		Labels: map[string]string{
			"cronado.backup.enabled":   "true",
			"cronado.backup.schedule":  "@every 1h",
			"cronado.backup.cmd":       "pg_dump mydb",
			"cronado.backup.timeout":   "5m",
			"cronado.cleanup.enabled":  "true",
			"cronado.cleanup.schedule": "0 3 * * *",
			"cronado.cleanup.cmd":      "rm -rf /tmp/*",
			"cronado.cleanup.timeout":  "2m",
		},
	}

	result := parseCronsFromContainer(container, "cronado")

	if len(result) != 2 {
		t.Fatalf("expected 2 cron jobs, got %d", len(result))
	}

	backup := result["backup"]
	if backup.Timeout != 5*time.Minute {
		t.Errorf("backup timeout = %v, want 5m", backup.Timeout)
	}

	cleanup := result["cleanup"]
	if cleanup.Timeout != 2*time.Minute {
		t.Errorf("cleanup timeout = %v, want 2m", cleanup.Timeout)
	}
}

func TestParseCronsFromContainer_DisabledJob(t *testing.T) {
	container := &Container{
		ID:   "abc123456789",
		Name: "test-container",
		Labels: map[string]string{
			"cronado.backup.enabled":  "false",
			"cronado.backup.schedule": "@every 1h",
			"cronado.backup.cmd":      "echo hello",
		},
	}

	result := parseCronsFromContainer(container, "cronado")

	if len(result) != 1 {
		t.Fatalf("expected 1 cron job, got %d", len(result))
	}

	job := result["backup"]
	if job.Enabled {
		t.Error("expected job to be disabled")
	}
}

func TestParseCronsFromContainer_NoRelevantLabels(t *testing.T) {
	container := &Container{
		ID:   "abc123456789",
		Name: "test-container",
		Labels: map[string]string{
			"app":     "my-app",
			"version": "1.0.0",
		},
	}

	result := parseCronsFromContainer(container, "cronado")

	if len(result) != 0 {
		t.Errorf("expected 0 cron jobs, got %d", len(result))
	}
}

func TestParseCronsFromContainer_InvalidLabelFormat(t *testing.T) {
	container := &Container{
		ID:   "abc123456789",
		Name: "test-container",
		Labels: map[string]string{
			"cronado.invalid": "value", // Missing property part
		},
	}

	result := parseCronsFromContainer(container, "cronado")

	if len(result) != 0 {
		t.Errorf("expected 0 cron jobs, got %d", len(result))
	}
}

func TestParseCronsFromContainer_CustomPrefix(t *testing.T) {
	container := &Container{
		ID:   "abc123456789",
		Name: "test-container",
		Labels: map[string]string{
			"myapp.backup.enabled":  "true",
			"myapp.backup.schedule": "@every 1h",
			"myapp.backup.cmd":      "echo hello",
		},
	}

	result := parseCronsFromContainer(container, "myapp")

	if len(result) != 1 {
		t.Fatalf("expected 1 cron job, got %d", len(result))
	}
}

func TestParseCronsFromContainer_MissingRequired(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
	}{
		{
			name: "missing schedule",
			labels: map[string]string{
				"cronado.job.enabled": "true",
				"cronado.job.cmd":     "echo hello",
			},
		},
		{
			name: "missing cmd",
			labels: map[string]string{
				"cronado.job.enabled":  "true",
				"cronado.job.schedule": "@every 1h",
			},
		},
		{
			name: "missing enabled",
			labels: map[string]string{
				"cronado.job.schedule": "@every 1h",
				"cronado.job.cmd":      "echo hello",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := &Container{
				ID:     "abc123456789",
				Name:   "test-container",
				Labels: tt.labels,
			}

			result := parseCronsFromContainer(container, "cronado")

			if len(result) != 0 {
				t.Errorf("expected 0 cron jobs (missing required field), got %d", len(result))
			}
		})
	}
}

func TestParseCronsFromContainer_InvalidJobNames(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   int
	}{
		{
			name: "special characters rejected",
			labels: map[string]string{
				"cronado.../etc/passwd.enabled":  "true",
				"cronado.../etc/passwd.schedule": "@daily",
				"cronado.../etc/passwd.cmd":      "echo hack",
			},
			want: 0,
		},
		{
			name: "semicolon rejected",
			labels: map[string]string{
				"cronado.job;rm -rf.enabled":  "true",
				"cronado.job;rm -rf.schedule": "@daily",
				"cronado.job;rm -rf.cmd":      "echo hack",
			},
			want: 0,
		},
		{
			name: "valid name with hyphens and underscores",
			labels: map[string]string{
				"cronado.my-backup_job.enabled":  "true",
				"cronado.my-backup_job.schedule": "@daily",
				"cronado.my-backup_job.cmd":      "echo ok",
			},
			want: 1,
		},
		{
			name: "name starting with hyphen rejected",
			labels: map[string]string{
				"cronado.-invalid.enabled":  "true",
				"cronado.-invalid.schedule": "@daily",
				"cronado.-invalid.cmd":      "echo bad",
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := &Container{
				ID:     "abc123456789",
				Name:   "test-container",
				Labels: tt.labels,
			}
			result := parseCronsFromContainer(container, "cronado")
			if len(result) != tt.want {
				t.Errorf("expected %d cron jobs, got %d", tt.want, len(result))
			}
		})
	}
}

// TestListenForEvents_ScansAfterSubscribing checks that the container scan
// runs only once the event stream is subscribed, so a container started
// during the scan is not missed.
func TestListenForEvents_ScansAfterSubscribing(t *testing.T) {
	var subscribed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1.47/events" {
			http.NotFound(w, r)
			return
		}
		subscribed.Store(true)
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	cli, err := client.NewClientWithOpts(client.WithHost("tcp://"+server.Listener.Addr().String()), client.WithVersion("1.47"))
	if err != nil {
		t.Fatal(err)
	}
	previous := dockerClient
	dockerClient = cli
	t.Cleanup(func() { dockerClient = previous })

	stop := make(chan os.Signal, 1)
	scannedAfterSubscribe := make(chan bool, 1)
	done := make(chan bool, 1)
	go func() {
		_, stopped := listenForEvents(filters.NewArgs(), stop, func() {
			scannedAfterSubscribe <- subscribed.Load()
		})
		done <- stopped
	}()

	select {
	case ok := <-scannedAfterSubscribe:
		if !ok {
			t.Error("containers were scanned before the event stream was subscribed")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("onSubscribed was not called")
	}

	stop <- os.Interrupt
	select {
	case stopped := <-done:
		if !stopped {
			t.Error("listenForEvents did not report the stop signal")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("listenForEvents did not return after the stop signal")
	}
}

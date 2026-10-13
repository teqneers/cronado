package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/teqneers/cronado/internal/config"
	"github.com/teqneers/cronado/internal/context"
	"github.com/teqneers/cronado/internal/domain"
)

// mockCronJobManager implements context.CronJobManager for testing
type mockCronJobManager struct {
	jobs []domain.CronJob
}

func (m *mockCronJobManager) Add(_ interface{}, _ interface{}) error { return nil }
func (m *mockCronJobManager) Remove(_ string)                        {}
func (m *mockCronJobManager) RemoveJob(_ interface{})                {}
func (m *mockCronJobManager) IsRegistered(_ string) bool             { return false }
func (m *mockCronJobManager) Shutdown()                              {}

func (m *mockCronJobManager) GetAll() []interface{} {
	result := make([]interface{}, len(m.jobs))
	for i, job := range m.jobs {
		result[i] = job
	}
	return result
}

func setupTestRouter(manager *mockCronJobManager) *gin.Engine {
	gin.SetMode(gin.TestMode)

	context.AppCtx = &context.AppContext{
		Config: &config.Config{
			Metrics: config.Metrics{
				Enabled: false,
			},
		},
		CronJobManager: manager,
	}

	return SetupRouter()
}

func TestGetCronJobList_Empty(t *testing.T) {
	manager := &mockCronJobManager{jobs: []domain.CronJob{}}
	router := setupTestRouter(manager)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/cron-job", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result []domain.CronListItem
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 items, got %d", len(result))
	}
}

func TestGetCronJobList_WithJobs(t *testing.T) {
	container := &domain.Container{
		ID:   "abcdef123456",
		Name: "test-container",
	}

	manager := &mockCronJobManager{
		jobs: []domain.CronJob{
			{
				ID:        "abc-backup",
				Name:      "backup",
				Container: container,
				Enabled:   true,
				Schedule:  "@every 1h",
				Command:   "pg_dump mydb",
				User:      "postgres",
				Status:    domain.StatusIdle,
			},
		},
	}
	router := setupTestRouter(manager)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/cron-job", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result []domain.CronListItem
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result))
	}

	if result[0].CronJob.Name != "backup" {
		t.Errorf("expected job name %q, got %q", "backup", result[0].CronJob.Name)
	}
	if result[0].ContainerID != container.ID {
		t.Errorf("expected container ID %q, got %q", container.ID, result[0].ContainerID)
	}
	if result[0].ContainerName != container.Name {
		t.Errorf("expected container name %q, got %q", container.Name, result[0].ContainerName)
	}
}

func TestSetupRouter_MetricsDisabled(t *testing.T) {
	manager := &mockCronJobManager{jobs: []domain.CronJob{}}
	router := setupTestRouter(manager)

	// Metrics endpoint should not exist
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/metrics", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for disabled metrics, got %d", w.Code)
	}
}

func TestSetupRouter_MetricsEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	context.AppCtx = &context.AppContext{
		Config: &config.Config{
			Metrics: config.Metrics{
				Enabled:  true,
				Endpoint: "/metrics",
			},
		},
		CronJobManager: &mockCronJobManager{jobs: []domain.CronJob{}},
	}

	router := SetupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/metrics", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for enabled metrics, got %d", w.Code)
	}
}

func TestGetCronJobList_ContentType(t *testing.T) {
	manager := &mockCronJobManager{jobs: []domain.CronJob{}}
	router := setupTestRouter(manager)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/cron-job", nil)
	router.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}
}

func TestInvalidRoute_Returns404(t *testing.T) {
	manager := &mockCronJobManager{jobs: []domain.CronJob{}}
	router := setupTestRouter(manager)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/nonexistent", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func getSingleJob(t *testing.T, router *gin.Engine, token string) domain.CronListItem {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/cron-job", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	var result []domain.CronListItem
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result))
	}
	return result[0]
}

func TestGetCronJobList_RedactsCommandWithoutToken(t *testing.T) {
	manager := &mockCronJobManager{jobs: []domain.CronJob{
		{ID: "abc-backup", Name: "backup", Command: "mysqldump -psecret db"},
	}}
	router := setupTestRouter(manager)

	item := getSingleJob(t, router, "")
	if item.CronJob.Command != RedactedCommand {
		t.Errorf("expected redacted command, got %q", item.CronJob.Command)
	}
}

func TestGetCronJobList_ShowsCommandWithToken(t *testing.T) {
	manager := &mockCronJobManager{jobs: []domain.CronJob{
		{ID: "abc-backup", Name: "backup", Command: "mysqldump -psecret db"},
	}}
	setupTestRouter(manager)
	context.AppCtx.Config.ServerConfig.APIToken = "s3cret"
	router := SetupRouter()

	item := getSingleJob(t, router, "s3cret")
	if item.CronJob.Command != "mysqldump -psecret db" {
		t.Errorf("expected full command, got %q", item.CronJob.Command)
	}
}

func TestHealthz_UnauthenticatedAndUnhealthyWithoutDocker(t *testing.T) {
	setupTestRouter(&mockCronJobManager{})
	context.AppCtx.Config.ServerConfig.APIToken = "s3cret"
	router := SetupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	router.ServeHTTP(w, req)

	// No auth header, yet not 401: the health check is public. No Docker
	// client in tests, so it reports unhealthy.
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	var status domain.HealthStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if status.Docker != "unreachable" || status.Status != "unhealthy" {
		t.Errorf("status = %+v", status)
	}
}

func TestGetCronJobList_RedactsPingURLWithoutToken(t *testing.T) {
	manager := &mockCronJobManager{jobs: []domain.CronJob{
		{ID: "abc-backup", Name: "backup", Command: "x", PingURL: "https://hc-ping.com/secret-uuid"},
	}}
	router := setupTestRouter(manager)

	item := getSingleJob(t, router, "")
	if item.CronJob.PingURL != RedactedValue {
		t.Errorf("expected redacted ping URL, got %q", item.CronJob.PingURL)
	}
}

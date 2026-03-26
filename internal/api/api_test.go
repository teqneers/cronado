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
func (m *mockCronJobManager) Remove(_ string)                       {}
func (m *mockCronJobManager) RemoveJob(_ interface{})               {}
func (m *mockCronJobManager) IsRegistered(_ string) bool            { return false }
func (m *mockCronJobManager) Shutdown()                             {}

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

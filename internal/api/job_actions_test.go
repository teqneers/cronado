package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/teqneers/cronado/internal/config"
	"github.com/teqneers/cronado/internal/context"
	"github.com/teqneers/cronado/internal/domain"
)

const testToken = "secret-token"

func setupActionRouter(manager *mockCronJobManager, token string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	context.AppCtx = &context.AppContext{
		Config:         &config.Config{ServerConfig: config.ServerConfig{APIToken: token}},
		CronJobManager: manager,
	}
	return SetupRouter()
}

func actionManager() *mockCronJobManager {
	return &mockCronJobManager{jobs: []domain.CronJob{{
		ID:        "abcdef123456-backup",
		Name:      "backup",
		Container: &domain.Container{ID: "abcdef123456", Name: "db"},
		Schedule:  "@daily",
		Command:   "pg_dump",
		Status:    domain.StatusIdle,
	}}}
}

func post(router *gin.Engine, path, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	router.ServeHTTP(w, req)
	return w
}

func TestJobActions_DisabledWithoutToken(t *testing.T) {
	manager := actionManager()
	router := setupActionRouter(manager, "")

	for _, action := range []string{"run", "pause", "resume"} {
		w := post(router, "/api/cron-job/abcdef123456-backup/"+action, "")
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "server.api_token") {
			t.Errorf("%s: status %d body %s, want 403 with hint", action, w.Code, w.Body.String())
		}
	}
	if len(manager.ran) != 0 || len(manager.paused) != 0 {
		t.Error("manager was called without a token")
	}
}

func TestJobActions_WrongToken(t *testing.T) {
	router := setupActionRouter(actionManager(), testToken)
	if w := post(router, "/api/cron-job/abcdef123456-backup/run", "wrong"); w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestRunCronJob(t *testing.T) {
	manager := actionManager()
	router := setupActionRouter(manager, testToken)

	w := post(router, "/api/cron-job/abcdef123456-backup/run", testToken)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (%s)", w.Code, w.Body.String())
	}
	if len(manager.ran) != 1 || manager.ran[0] != "abcdef123456-backup" {
		t.Errorf("ran = %v", manager.ran)
	}
	var item domain.CronListItem
	if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil || item.CronJob.Command != "pg_dump" {
		t.Errorf("body = %s (err %v), want job with command", w.Body.String(), err)
	}
}

func TestRunCronJob_Errors(t *testing.T) {
	manager := actionManager()
	router := setupActionRouter(manager, testToken)

	if w := post(router, "/api/cron-job/missing/run", testToken); w.Code != http.StatusNotFound {
		t.Errorf("unknown job: status = %d, want 404", w.Code)
	}

	manager.runErr = domain.ErrJobBusy
	if w := post(router, "/api/cron-job/abcdef123456-backup/run", testToken); w.Code != http.StatusConflict {
		t.Errorf("busy job: status = %d, want 409", w.Code)
	}

	manager.runErr = domain.ErrShuttingDown
	if w := post(router, "/api/cron-job/abcdef123456-backup/run", testToken); w.Code != http.StatusServiceUnavailable {
		t.Errorf("shutting down: status = %d, want 503", w.Code)
	}
}

func TestPauseAndResumeCronJob(t *testing.T) {
	manager := actionManager()
	router := setupActionRouter(manager, testToken)

	w := post(router, "/api/cron-job/abcdef123456-backup/pause", testToken)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"paused":true`) {
		t.Errorf("pause: status %d body %s", w.Code, w.Body.String())
	}
	w = post(router, "/api/cron-job/abcdef123456-backup/resume", testToken)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"paused":false`) {
		t.Errorf("resume: status %d body %s", w.Code, w.Body.String())
	}
	if w := post(router, "/api/cron-job/missing/pause", testToken); w.Code != http.StatusNotFound {
		t.Errorf("unknown job: status = %d, want 404", w.Code)
	}
}

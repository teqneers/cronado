package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupAuthTestRouter(token string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/test", BearerAuthMiddleware(token), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return r
}

func TestBearerAuth_NoTokenConfigured_AllowsAll(t *testing.T) {
	r := setupAuthTestRouter("")

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 when no token configured, got %d", w.Code)
	}
}

func TestBearerAuth_ValidToken(t *testing.T) {
	r := setupAuthTestRouter("secret-token")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with valid token, got %d", w.Code)
	}
}

func TestBearerAuth_InvalidToken(t *testing.T) {
	r := setupAuthTestRouter("secret-token")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with invalid token, got %d", w.Code)
	}
}

func TestBearerAuth_MissingHeader(t *testing.T) {
	r := setupAuthTestRouter("secret-token")

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with missing header, got %d", w.Code)
	}
}

func TestBearerAuth_WrongScheme(t *testing.T) {
	r := setupAuthTestRouter("secret-token")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Basic secret-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with wrong auth scheme, got %d", w.Code)
	}
}

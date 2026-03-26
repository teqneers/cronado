package notification

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/teqneers/cronado/internal/config"
	"github.com/teqneers/cronado/internal/context"
)

func TestSendNtfy_Disabled(t *testing.T) {
	context.AppCtx = &context.AppContext{
		Config: &config.Config{
			Notify: config.Notify{
				Ntfy: config.NtfyConfig{
					Enabled: false,
					Server:  "http://localhost",
					Topic:   "test",
				},
			},
		},
	}

	err := SendNtfy("test subject", "test body")
	if err != nil {
		t.Errorf("expected no error when disabled, got %v", err)
	}
}

func TestSendNtfy_EmptyTopic(t *testing.T) {
	context.AppCtx = &context.AppContext{
		Config: &config.Config{
			Notify: config.Notify{
				Ntfy: config.NtfyConfig{
					Enabled: true,
					Server:  "http://localhost",
					Topic:   "",
				},
			},
		},
	}

	err := SendNtfy("test subject", "test body")
	if err != nil {
		t.Errorf("expected no error with empty topic, got %v", err)
	}
}

func TestSendNtfy_Success(t *testing.T) {
	var receivedTitle string
	var receivedBody string
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTitle = r.Header.Get("Title")
		receivedAuth = r.Header.Get("Authorization")

		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		receivedBody = string(body)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	context.AppCtx = &context.AppContext{
		Config: &config.Config{
			Notify: config.Notify{
				Ntfy: config.NtfyConfig{
					Enabled: true,
					Server:  server.URL,
					Topic:   "test-topic",
					Token:   "my-token",
				},
			},
		},
	}

	err := SendNtfy("Test Alert", "Something failed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedTitle != "Test Alert" {
		t.Errorf("title = %q, want %q", receivedTitle, "Test Alert")
	}
	if receivedBody != "Something failed" {
		t.Errorf("body = %q, want %q", receivedBody, "Something failed")
	}
	if receivedAuth != "Bearer my-token" {
		t.Errorf("auth = %q, want %q", receivedAuth, "Bearer my-token")
	}
}

func TestSendNtfy_NoAuth(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	context.AppCtx = &context.AppContext{
		Config: &config.Config{
			Notify: config.Notify{
				Ntfy: config.NtfyConfig{
					Enabled: true,
					Server:  server.URL,
					Topic:   "test-topic",
					Token:   "",
				},
			},
		},
	}

	err := SendNtfy("Test", "Body")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedAuth != "" {
		t.Errorf("expected no auth header, got %q", receivedAuth)
	}
}

func TestSendNtfy_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	context.AppCtx = &context.AppContext{
		Config: &config.Config{
			Notify: config.Notify{
				Ntfy: config.NtfyConfig{
					Enabled: true,
					Server:  server.URL,
					Topic:   "test-topic",
				},
			},
		},
	}

	err := SendNtfy("Test", "Body")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestSendNtfy_TrailingSlash(t *testing.T) {
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	context.AppCtx = &context.AppContext{
		Config: &config.Config{
			Notify: config.Notify{
				Ntfy: config.NtfyConfig{
					Enabled: true,
					Server:  server.URL + "/",
					Topic:   "my-topic",
				},
			},
		},
	}

	err := SendNtfy("Test", "Body")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedPath != "/my-topic" {
		t.Errorf("path = %q, want %q", receivedPath, "/my-topic")
	}
}

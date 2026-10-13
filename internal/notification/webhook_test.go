package notification

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/teqneers/cronado/internal/config"
	"github.com/teqneers/cronado/internal/context"
)

func setWebhookConfig(cfg config.WebhookConfig) {
	context.AppCtx = &context.AppContext{
		Config: &config.Config{Notify: config.Notify{Webhook: cfg}},
	}
}

func TestSendWebhook_Disabled(t *testing.T) {
	setWebhookConfig(config.WebhookConfig{Enabled: false, URL: "http://127.0.0.1:1"})
	if err := SendWebhook("s", "b"); err != nil {
		t.Errorf("expected no error when disabled, got %v", err)
	}
}

func TestSendWebhook_Formats(t *testing.T) {
	tests := []struct {
		format string
		want   map[string]any
	}{
		{format: "", want: map[string]any{"title": "Job failed", "message": "exit 1"}},
		{format: "generic", want: map[string]any{"title": "Job failed", "message": "exit 1"}},
		{format: "slack", want: map[string]any{"text": "*Job failed*\nexit 1"}},
		{format: "discord", want: map[string]any{"content": "**Job failed**\nexit 1"}},
		{format: "gotify", want: map[string]any{"title": "Job failed", "message": "exit 1", "priority": float64(5)}},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			var got map[string]any
			var contentType string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				contentType = r.Header.Get("Content-Type")
				data, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(data, &got)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			setWebhookConfig(config.WebhookConfig{Enabled: true, URL: server.URL, Format: tt.format})
			if err := SendWebhook("Job failed", "exit 1"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if contentType != "application/json" {
				t.Errorf("Content-Type = %q", contentType)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("payload = %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("payload[%q] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestSendWebhook_UnknownFormat(t *testing.T) {
	setWebhookConfig(config.WebhookConfig{Enabled: true, URL: "http://127.0.0.1:1", Format: "teams"})
	err := SendWebhook("s", "b")
	if err == nil || !strings.Contains(err.Error(), "unknown webhook format") {
		t.Errorf("expected unknown format error, got %v", err)
	}
}

func TestSendWebhook_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad payload"))
	}))
	defer server.Close()

	setWebhookConfig(config.WebhookConfig{Enabled: true, URL: server.URL})
	err := SendWebhook("s", "b")
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Errorf("expected status error, got %v", err)
	}
}

func TestWebhookPayload_DiscordTruncates(t *testing.T) {
	data, err := webhookPayload(WebhookFormatDiscord, "s", strings.Repeat("x", 3000))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	_ = json.Unmarshal(data, &got)
	if len(got["content"]) > discordMaxContent {
		t.Errorf("content length = %d, want <= %d", len(got["content"]), discordMaxContent)
	}
}

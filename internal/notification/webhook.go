package notification

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	appctx "github.com/teqneers/cronado/internal/context"
)

// Supported webhook payload formats.
const (
	WebhookFormatGeneric = "generic"
	WebhookFormatSlack   = "slack"
	WebhookFormatDiscord = "discord"
	WebhookFormatGotify  = "gotify"
)

// discordMaxContent is Discord's limit for the content field of a message.
const discordMaxContent = 2000

// SendWebhook posts a notification to the configured webhook URL, using the
// payload shape of the configured format.
func SendWebhook(subject, body string) error {
	cfg := appctx.AppCtx.Config.Notify.Webhook
	if !cfg.Enabled || cfg.URL == "" {
		return nil
	}

	payload, err := webhookPayload(cfg.Format, subject, body)
	if err != nil {
		return err
	}

	// Errors below never include cfg.URL: webhook URLs usually embed a secret.
	req, err := http.NewRequest(http.MethodPost, cfg.URL, bytes.NewReader(payload))
	if err != nil {
		return errors.New("webhook create request: invalid webhook URL")
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return fmt.Errorf("webhook send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("webhook unexpected status %d: %s", resp.StatusCode, string(data))
	}
	return nil
}

// webhookPayload builds the JSON body for the given format.
func webhookPayload(format, subject, body string) ([]byte, error) {
	var payload any
	switch strings.ToLower(format) {
	case "", WebhookFormatGeneric:
		payload = map[string]string{"title": subject, "message": body}
	case WebhookFormatSlack:
		// Also accepted by Mattermost and Rocket.Chat incoming webhooks.
		payload = map[string]string{"text": fmt.Sprintf("*%s*\n%s", subject, body)}
	case WebhookFormatDiscord:
		content := fmt.Sprintf("**%s**\n%s", subject, body)
		if len(content) > discordMaxContent {
			content = strings.ToValidUTF8(content[:discordMaxContent-3], "") + "..."
		}
		payload = map[string]string{"content": content}
	case WebhookFormatGotify:
		payload = map[string]any{"title": subject, "message": body, "priority": 5}
	default:
		return nil, fmt.Errorf("unknown webhook format %q (expected generic, slack, discord or gotify)", format)
	}
	return json.Marshal(payload)
}

package notification

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	appctx "github.com/teqneers/cronado/internal/context"
)

// SendNtfy sends a notification via ntfy.sh (or compatible) service.
func SendNtfy(subject, body string) error {
	cfg := appctx.AppCtx.Config.Notify.Ntfy

	if !cfg.Enabled || cfg.Topic == "" {
		return nil
	}

	endpoint := fmt.Sprintf("%s/%s", strings.TrimRight(cfg.Server, "/"), cfg.Topic)
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("ntfy create request: %w", err)
	}

	// set title header
	req.Header.Set("Title", subject)

	// set authorization if token is provided
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Drop the URL from the error: on public servers the topic is the secret.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return fmt.Errorf("ntfy send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("ntfy unexpected status %d: %s", resp.StatusCode, string(data))
	}

	return nil
}

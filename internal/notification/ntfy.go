package notification

import (
	"fmt"
	"io"
	"net/http"
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

	url := fmt.Sprintf("%s/%s", strings.TrimRight(cfg.Server, "/"), cfg.Topic)
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
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
		return fmt.Errorf("ntfy send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ntfy unexpected status %d: %s", resp.StatusCode, string(data))
	}

	return nil
}

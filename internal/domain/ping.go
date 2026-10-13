package domain

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Healthchecks.io-compatible ping suffixes, appended to the job's ping URL.
const (
	pingStart   = "/start"
	pingSuccess = ""
	pingFail    = "/fail"
)

// maxPingBody caps the output sent with a failure ping.
const maxPingBody = 10 * 1024

// pingClient sends dead man's switch pings. Redirects are not followed, so a
// ping URL cannot be used to reach other hosts.
var pingClient = &http.Client{
	Timeout: 5 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// validatePingURL checks that a ping URL is an absolute http(s) URL.
func validatePingURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid ping_url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid ping_url %q: scheme must be http or https", raw)
	}
	if u.Host == "" {
		return fmt.Errorf("invalid ping_url %q: missing host", raw)
	}
	return nil
}

// sendPing notifies a Healthchecks.io-compatible endpoint. Failures are logged
// and never affect the job itself.
func sendPing(job *CronJob, suffix, body string) {
	if job.PingURL == "" {
		return
	}
	u, err := url.Parse(job.PingURL)
	if err != nil {
		slog.Warn("Invalid ping URL", "job_id", job.ID, "error", err)
		return
	}
	u.Path = strings.TrimRight(u.Path, "/") + suffix

	if len(body) > maxPingBody {
		body = body[len(body)-maxPingBody:]
	}

	ctx, cancel := context.WithTimeout(context.Background(), pingClient.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(body))
	if err != nil {
		slog.Warn("Failed to create ping request", "job_id", job.ID, "error", err)
		return
	}
	resp, err := pingClient.Do(req)
	if err != nil {
		slog.Warn("Failed to send ping", "job_id", job.ID, "error", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Warn("Ping returned unexpected status", "job_id", job.ID, "status", resp.StatusCode)
	}
}

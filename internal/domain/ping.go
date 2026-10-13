package domain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	cronadoCtx "github.com/teqneers/cronado/internal/context"
)

// Healthchecks.io-compatible ping suffixes, appended to the job's ping URL.
const (
	pingStart   = "/start"
	pingSuccess = ""
	pingFail    = "/fail"
)

// maxPingBody caps the output sent with a failure ping.
const maxPingBody = 10 * 1024

// errPingTargetBlocked is returned when a ping URL resolves to a private,
// loopback or otherwise internal address and those are not allowed.
var errPingTargetBlocked = errors.New("ping target is a private or internal address (set ping.allow_private_networks to allow)")

// allowPrivatePingTargets reports whether pings may reach internal addresses.
// Ping URLs come from container labels, so by default they must not be usable
// to reach services on cronado's own network, such as a Docker socket proxy.
var allowPrivatePingTargets = func() bool {
	return cronadoCtx.AppCtx != nil && cronadoCtx.AppCtx.Config != nil && cronadoCtx.AppCtx.Config.PingAllowPrivateNetworks
}

// cgnatRange is the shared address space (RFC 6598), which IsPrivate omits.
var cgnatRange = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// isInternalIP reports whether ip is not a public unicast address.
func isInternalIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() || cgnatRange.Contains(ip)
}

// pingClient sends dead man's switch pings. The address check runs on the
// resolved IP at connect time, so DNS tricks cannot bypass it. Redirects are
// not followed and no proxy is used, for the same reason.
var pingClient = &http.Client{
	Timeout: 5 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
	Transport: &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout: 5 * time.Second,
			Control: func(_, address string, _ syscall.RawConn) error {
				if allowPrivatePingTargets() {
					return nil
				}
				host, _, err := net.SplitHostPort(address)
				if err != nil {
					return errPingTargetBlocked
				}
				if ip := net.ParseIP(host); ip == nil || isInternalIP(ip) {
					return errPingTargetBlocked
				}
				return nil
			},
		}).DialContext,
	},
}

// validatePingURL checks that a ping URL is an absolute http(s) URL. Errors
// never contain the URL, because ping URLs embed a secret check ID.
func validatePingURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("invalid ping_url: not a valid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("invalid ping_url: scheme must be http or https")
	}
	if u.Host == "" {
		return errors.New("invalid ping_url: missing host")
	}
	return nil
}

// stripURL removes the request URL from an HTTP client error, so that secret
// URLs do not end up in logs.
func stripURL(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Errorf("%s request failed: %w", urlErr.Op, urlErr.Err)
	}
	return err
}

// sendPing notifies a Healthchecks.io-compatible endpoint. Failures are logged
// and never affect the job itself.
func sendPing(job *CronJob, suffix, body string) {
	if job.PingURL == "" {
		return
	}
	u, err := url.Parse(job.PingURL)
	if err != nil {
		slog.Warn("Invalid ping URL", "job_id", job.ID)
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
		slog.Warn("Failed to create ping request", "job_id", job.ID, "error", stripURL(err))
		return
	}
	resp, err := pingClient.Do(req)
	if err != nil {
		slog.Warn("Failed to send ping", "job_id", job.ID, "error", stripURL(err))
		return
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Warn("Ping returned unexpected status", "job_id", job.ID, "status", resp.StatusCode)
	}
}

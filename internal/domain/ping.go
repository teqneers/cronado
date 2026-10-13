package domain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
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

// blockedPingPrefixes are the IANA special-purpose ranges a ping must not
// reach: private, loopback, link-local, shared, reserved, documentation and
// multicast space, plus IPv6 transition ranges (NAT64, 6to4, Teredo,
// IPv4-compatible) that can embed an internal IPv4 address.
var blockedPingPrefixes = mustParsePrefixes(
	// IPv4
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
	"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "192.168.0.0/16",
	"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
	// IPv6
	"::/96", "::1/128", "64:ff9b::/96", "64:ff9b:1::/48", "100::/64", "2001::/23",
	"2001:db8::/32", "2002::/16", "fc00::/7", "fe80::/10", "fec0::/10", "ff00::/8",
)

func mustParsePrefixes(prefixes ...string) []netip.Prefix {
	result := make([]netip.Prefix, len(prefixes))
	for i, p := range prefixes {
		result[i] = netip.MustParsePrefix(p)
	}
	return result
}

// isInternalIP reports whether ip is not a public unicast address.
// IPv4-mapped IPv6 addresses (::ffff:a.b.c.d) are checked as IPv4.
func isInternalIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	addr = addr.Unmap()
	for _, prefix := range blockedPingPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
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

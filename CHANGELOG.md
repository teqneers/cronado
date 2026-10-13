# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] - Unreleased

### Changed

- **Breaking:** commands now run as the container's own `USER` when no `user` label is set (previously `root`). Add `cronado.<job>.user=root` to jobs that need root
- Without `server.api_token`, job commands are redacted in `/api/cron-job` responses, and a warning is logged at startup when the server listens on a non-loopback address
- A broken Docker event stream is now reconnected with backoff and the job list resynced, instead of exiting immediately. Cronado still notifies and exits if Docker stays unreachable for about a minute
- Cronado only subscribes to container start, stop, die and destroy events
- Failure notification titles now include the container name (`Cron Job Failed: <job> on <container>`)

### Security

- Commands that exceed their timeout are now killed inside the container (`timeout -s KILL`); previously they kept running while the next run started
- Captured command output is capped at 64 KiB per stream to prevent memory exhaustion
- `min_schedule_interval` now also applies to 6-field schedules with a seconds field
- API token comparison is constant-time
- HTTP server now has read, write and idle timeouts
- Failure notifications are throttled per job and container, so a same-named job in another container can no longer suppress alerts
- CI grants `packages: write` only to the nightly image job
- SECURITY.md: vulnerabilities are now reported via private GitHub Security Advisories

### Fixed

- 6-field schedules (with seconds) passed validation but were never registered
- Invalid cron expressions (e.g. `99 * * * *`) are now rejected when labels are parsed, with the parser's error message
- `--config` flag was ignored
- `cronado cron-job list` now sends the API token and reports HTTP errors
- `cronado config` printed an empty log level
- Prometheus series of removed jobs are deleted instead of accumulating
- ntfy error responses are read with a size limit

## [1.0.4] - 2026-10-13

### Security

- Pinned Go toolchain to 1.26.9 to fix CVE-2026-78667, CVE-2026-78669 and CVE-2026-97031 (the v1.0.3 release binary was built with Go 1.26.8)
- CI and release workflows now take the Go version from `go.mod`
- Release Docker image now runs `apk upgrade` to pick up patched Alpine packages

## [1.0.3] - 2026-10-13

### Security

- Upgraded Go from 1.25 to 1.26 to fix multiple standard library CVEs (net/http, crypto/tls, crypto/x509, net/mail, encoding/asn1, and others)
- Bumped `golang.org/x/net` to v0.61.0, `golang.org/x/crypto` to v0.58.0, `golang.org/x/text` to v0.43.0, and `quic-go` to v0.63.0
- Docker runtime image now runs `apk upgrade` to pick up patched OpenSSL (`libssl3`/`libcrypto3` 3.5.9)

### Changed

- "No user specified, defaulting to 'root'" warning is now logged once when a job is registered instead of on every execution

## [1.0.2] - 2026-04-26

### Added

- Optional Bearer token authentication for API and metrics endpoints (`CRONADO_SERVER_API_TOKEN`)
- Configurable minimum schedule interval to prevent rapid-fire `@every` schedules (`CRONADO_MIN_SCHEDULE_INTERVAL`, default `1m`)
- Configurable maximum job timeout cap (`CRONADO_MAX_TIMEOUT`, default `12h`)
- Job name validation — only alphanumeric characters, hyphens, and underscores are accepted
- TLS enforcement for SMTP email notifications (`CRONADO_NOTIFY_EMAIL_REQUIRE_TLS`, default `true`)
- `.dockerignore` to reduce Docker build context size

### Changed

- Email notifications now use the [go-mail](https://github.com/wneessen/go-mail) library, replacing the hand-rolled SMTP implementation
- Docker executor returns stdout/stderr directly instead of using shared buffers (thread safety)
- Default root user warning elevated from INFO to WARN log level
- Commands truncated to 80 characters in INFO logs; full command available at DEBUG level
- Docker base images pinned to `alpine:3` instead of `alpine:latest`

### Fixed

- CRLF header injection vulnerability in email notification subject lines
- Flaky `TestContainer_GetLabelsWithPrefix` test caused by map key collision

## [1.0.1] - 2026-04-25

### Fixed

- Preserve outer quotes in CronJobBuilder inputs and add regression tests

## [1.0.0] - 2025-03-25

### Added

- Label-based cron job scheduling for Docker containers
- Automatic container start/stop event detection
- Per-job configurable timeout via `timeout` label (e.g., `cronado.job.timeout=5m`)
- HTTP API endpoint (`GET /api/cron-job`) to list active jobs
- CLI command `cron-job list` to list active jobs
- Prometheus metrics: `cronado_scheduled_jobs`, `cronado_job_executions_total`, `cronado_job_duration_seconds`
- Email notifications (SMTP) on job failure
- [ntfy.sh](https://ntfy.sh) push notifications on job failure with per-subject throttling
- Docker daemon health monitoring with automatic restart
- Configuration via YAML file or environment variables (`CRONADO_` prefix)
- Multi-stage Docker image with non-root user

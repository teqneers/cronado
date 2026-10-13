# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

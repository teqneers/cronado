# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

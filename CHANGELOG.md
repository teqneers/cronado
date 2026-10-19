# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.2.0] - 2026-10-19

### Added

- `timezone` label to run a job's schedule in an IANA time zone, e.g. `cronado.<job>.timezone=Europe/Berlin`. Invalid names and a conflicting `CRON_TZ=` prefix in the schedule are rejected. See [Time zones](README.md#time-zones), including how daylight saving time affects jobs in the skipped or repeated hour
- `jitter` label to delay each run by a random amount up to the given duration, e.g. `cronado.<job>.jitter=2m`, so jobs sharing a schedule do not all start at once. The jitter must be shorter than the schedule interval. While waiting, the job's status is `pending`, and a shutdown cancels the wait. See [Spreading start times](README.md#spreading-start-times-jitter)
- `GET /api/label-issues` lists rejected jobs and ignored labels of running containers with the reason, and `cronado cron-job list` shows them. See [List Label Problems](README.md#list-label-problems)
- `cronado_label_issues` metric (labels `container_id`, `container_name`, `job_name`, `kind`) to alert on invalid labels
- `POST /api/cron-job/{id}/run`, `/pause` and `/resume` to run a job now or skip its scheduled runs, and matching `cronado cron-job run|pause|resume <job-id>` commands. They require `server.api_token` and return `403` without it. Pauses are kept in memory only. See [Run, Pause and Resume Jobs](README.md#run-pause-and-resume-jobs)
- `cronado_job_paused` metric and `paused` field in `/api/cron-job`
- `cronado validate` command to check cron job labels with the scheduler's own rules. `-f` checks the services of one or more Compose files without Docker access, e.g. in CI; without `-f`, it checks all running containers. Exits with `1` if any problem is found. See [Checking labels before deployment](README.md#checking-labels-before-deployment)
- `replicas` label to choose whether a job of a Compose service with several replicas runs on one replica (`once`, default) or on each (`all`). Jobs of the other replicas are shown as `standby` in the API and `cronado cron-job list`, and in the `cronado_job_standby` metric. See [Replicated services](README.md#replicated-services)
- Jobs in the config file: a `jobs:` list with `name`, `container` and the label properties, for containers whose labels you cannot change. Config jobs are marked with `"source": "config"` in the API and label problems and `from config file` in `cronado cron-job list`, and `cronado validate` checks them. See [Jobs in the config file](README.md#jobs-in-the-config-file)
- `image` label to run a job in a new container from an image instead of inside the labeled container, with `network`, `volumes` and `env.<NAME>` labels. The image is pulled if missing, and the container is removed after each run. Config file jobs take `image` instead of `container`. `image` labels must be enabled with `allow_label_image_jobs` (`CRONADO_ALLOW_LABEL_IMAGE_JOBS=true`), because images can carry labels. See [Jobs in their own container](README.md#jobs-in-their-own-container)

### Changed

- **Breaking:** jobs of a Compose service with several replicas now run on one replica only, the one with the lowest container number; the next replica takes over when it stops. Add `cronado.<job>.replicas=all` to jobs that must run in every replica
- On shutdown, cronado waits for running jobs for at most `shutdown_timeout` (`CRONADO_SHUTDOWN_TIMEOUT`, default `8s`, below Docker's 10s stop grace period) and logs the jobs it abandons. Previously it waited without a limit until Docker killed it. Manual runs are refused with `503` during shutdown. See [Stopping and updating cronado](README.md#stopping-and-updating-cronado)
- `cronado cron-job list` shows the job ID (needed for run, pause and resume) instead of the internal scheduler entry ID, and marks paused jobs
- `cronado cron-job list` connects via loopback when the server listens on `0.0.0.0` or `::`, like `cronado healthcheck`
- A job with an invalid label now reports all problems at once, including missing required labels, instead of only the first error
- Ignored labels (unknown property, invalid label format or job name) are logged with a consistent `Ignoring invalid cron job label` message

### Security

- GitHub Actions are pinned to commit SHAs, and Dependabot keeps them and the Go modules up to date

### Fixed

- Containers started while cronado is starting up are no longer missed until their next restart

## [1.1.0] - 2026-10-13

### Added

- Failure notifications include the last 20 lines of the command output (`CRONADO_NOTIFY_INCLUDE_OUTPUT`, default `true`)
- Webhook notifications with `generic`, `slack`, `discord` and `gotify` payload formats (`CRONADO_NOTIFY_WEBHOOK_*`)
- `ping_url` label for Healthchecks.io-compatible start, success and failure pings. Private and loopback targets are refused unless `CRONADO_PING_ALLOW_PRIVATE_NETWORKS=true`
- API returns `last_run`, `last_success`, `last_result`, `last_error`, `last_duration_seconds` and `next_run` per job
- Prometheus metrics `cronado_job_last_run_timestamp_seconds`, `cronado_job_last_success_timestamp_seconds`, `cronado_job_next_run_timestamp_seconds` and `cronado_job_running`
- Unauthenticated `GET /healthz` endpoint, `cronado healthcheck` command and Docker `HEALTHCHECK`

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
- ntfy errors no longer include the server URL and topic in logs

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

# Cronado

<p align="center">
  <img src="files/cronado-logo.png" alt="Cronado Logo" width="200">
</p>

[![CI](https://github.com/teqneers/cronado/actions/workflows/ci.yml/badge.svg)](https://github.com/teqneers/cronado/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/teqneers/cronado)](https://goreportcard.com/report/github.com/teqneers/cronado)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/teqneers/cronado)](go.mod)

A lightweight cron job scheduler for Docker containers. Cronado watches Docker events, detects containers with specific
labels, and automatically schedules commands to run inside them — no sidecars, no config files per container, just
labels.

> **Looking for an alternative
to [Ofelia](https://github.com/mcuadros/ofelia), [Chadburn](https://github.com/PremoWeb/chadburn), [Supercronic](https://github.com/aptible/supercronic),
or host-level crontab for Docker?** Cronado offers a modern, actively maintained approach with built-in metrics and
> notifications.

## Why Cronado?

|                                    | Cronado | [Ofelia](https://github.com/mcuadros/ofelia) | [Chadburn](https://github.com/PremoWeb/chadburn) | [Supercronic](https://github.com/aptible/supercronic) | Host crontab |
|------------------------------------|---------|----------------------------------------------|--------------------------------------------------|-------------------------------------------------------|--------------|
| No sidecar per container           | Yes     | Yes                                          | Yes                                              | No (runs inside)                                      | Yes          |
| Automatic container discovery      | Yes     | Yes                                          | Yes                                              | No                                                    | No           |
| Label-driven configuration         | Yes     | Yes                                          | Yes                                              | No                                                    | No           |
| Per-job timeout                    | Yes     | No                                           | No                                               | No                                                    | No           |
| Built-in Prometheus metrics        | Yes     | No                                           | No                                               | Partial                                               | No           |
| Failure notifications (email/ntfy) | Yes     | No                                           | Slack                                            | No                                                    | No           |
| Actively maintained                | Yes     | Limited                                      | Slow going                                       | Yes                                                   | N/A          |
| Docker daemon health monitoring    | Yes     | No                                           | No                                               | No                                                    | No           |

## Features

- Schedule commands inside Docker containers via container labels
- Automatic detection of container start/stop events
- Per-job configurable execution timeout
- Per-job time zones, so schedules follow local time including daylight saving time
- Per-job random start delay (jitter) to spread out jobs that share a schedule
- HTTP API to list active cron jobs with last result and next run, and the label problems of running containers
- Run a job on demand, or pause and resume it, via API and CLI
- Jobs of a Compose service with several replicas run on one replica only, with automatic takeover
- CLI for management (`cron` to start, `cron-job list` to inspect, `validate` to check labels, `healthcheck`)
- Prometheus metrics (`/metrics`), including last success and next run per job
- Failure notifications via email (SMTP), [ntfy.sh](https://ntfy.sh) and webhooks (Slack, Discord, Gotify, generic JSON), including the end of the command output
- [Healthchecks.io](https://healthchecks.io)-compatible pings per job, to catch jobs that silently stop running
- `/healthz` endpoint and Docker `HEALTHCHECK`
- Configurable via YAML or environment variables (`CRONADO_` prefix)
- Docker daemon health monitoring with automatic recovery

## Quick Start

### Docker (recommended)

```bash
docker run -d \
  --name cronado \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -e CRONADO_SERVER_HOST=0.0.0.0 \
  ghcr.io/teqneers/cronado:latest
```

> **Note:** `CRONADO_SERVER_HOST=0.0.0.0` is required when running in a container. The default `127.0.0.1`
> binds only to localhost inside the container, making the API unreachable from the host even with `-p 8080:8080`.

### Docker Compose

```yaml
services:
  cronado:
    image: ghcr.io/teqneers/cronado:latest
    container_name: cronado
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    ports:
      - "8080:8080"
    environment:
      CRONADO_SERVER_HOST: "0.0.0.0"
      CRONADO_LOG_LEVEL: info
```

### Build from Source

```bash
git clone https://github.com/teqneers/cronado.git
cd cronado
go build -o cronado main.go
./cronado cron
```

## How It Works

```
                    Docker Engine
                         |
          container start/stop events
                         |
                    +-----------+
                    |  Cronado  |
                    +-----------+
                    | Event     |----> Detect labels
                    | Listener  |      (cronado.*.schedule, etc.)
                    +-----------+
                         |
                    Register/remove
                    cron schedules
                         |
                    +-----------+
                    | Scheduler |----> docker exec into containers
                    +-----------+      on cron schedule
                         |
              +----------+----------+
              |          |          |
          Prometheus   Logs    Notifications
          /metrics              (email/ntfy)
```

## Scheduling Jobs via Container Labels

Add labels to your containers to define cron jobs. The default prefix is `cronado`:

```
cronado.<job-name>.enabled=true|false
cronado.<job-name>.schedule=@every 10s
cronado.<job-name>.cmd=echo hello
cronado.<job-name>.user=root
cronado.<job-name>.timezone=Europe/Berlin
cronado.<job-name>.timeout=5m
cronado.<job-name>.jitter=2m
cronado.<job-name>.ping_url=https://hc-ping.com/<uuid>
cronado.<job-name>.replicas=once|all
```

Each job needs at least `enabled`, `schedule`, and `cmd`. The prefix can be changed via `CRONADO_CRON_LABEL_PREFIX`.

### Example

Run a container with a cron job that logs timestamps every minute:

```bash
docker run -d \
  --name my-app \
  --label cronado.backup.enabled=true \
  --label cronado.backup.schedule="0 2 * * *" \
  --label cronado.backup.cmd="pg_dump -U postgres mydb > /backups/daily.sql" \
  --label cronado.backup.user=postgres \
  --label cronado.backup.timeout=10m \
  postgres:16
```

### Schedule Formats

Cronado supports standard cron expressions and convenience shortcuts:

| Format        | Example        | Description            |
|---------------|----------------|------------------------|
| Standard cron | `0 2 * * *`    | Daily at 2:00 AM       |
| 6-field cron  | `0 30 * * * *` | Every hour at :30 (first field is seconds) |
| Time zone     | `CRON_TZ=Europe/Berlin 0 2 * * *` | Daily at 2:00 AM Berlin time (or use the `timezone` label) |
| `@every`      | `@every 5m`    | Every 5 minutes        |
| `@hourly`     | `@hourly`      | Every hour             |
| `@daily`      | `@daily`       | Once a day at midnight |
| `@weekly`     | `@weekly`      | Once a week            |
| `@monthly`    | `@monthly`     | Once a month           |

### Time zones

By default, schedules run in cronado's own time zone. The cronado image uses UTC unless you set the `TZ` environment variable on the cronado container. In UTC, a job meant for "2:00 AM local time" moves by one hour twice a year when daylight saving time starts and ends.

Set the `timezone` label to run a job's schedule in a specific time zone:

```
cronado.backup.schedule=0 2 * * *
cronado.backup.timezone=Europe/Berlin
```

This job runs at 2:00 AM Berlin time all year: 00:00 UTC in summer and 01:00 UTC in winter.

- **Allowed values:** IANA time zone names such as `Europe/Berlin`, `America/New_York` or `UTC` ([list](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones)). Prefer region names over legacy names like `CET` or `EST`. Offsets like `+02:00` and abbreviations like `CEST` are rejected.
- **Invalid names** reject the job with an error in the log, like an invalid schedule.
- **Prefix form:** `CRON_TZ=Europe/Berlin 0 2 * * *` in the schedule does the same thing. Use one or the other: a job with both the label and a `CRON_TZ=` or `TZ=` prefix is rejected.
- **All jobs at once:** to change the default for every job, set `TZ=Europe/Berlin` on the cronado container instead of labelling each job.
- **`@every` schedules** run at fixed intervals and are not affected by the time zone.
- **API:** `schedule` shows the label value as written and `timezone` shows the zone. `next_run` includes its UTC offset, so it is unambiguous whichever zone it is shown in.

**Daylight saving time:** on the day the clocks go forward, times in the skipped hour do not exist, so a job scheduled then (for example `30 2 * * *` in `Europe/Berlin`) does not run that day. On the day the clocks go back, times in the repeated hour occur twice, so such a job runs twice. Avoid scheduling daily jobs between 2:00 and 3:00 AM in zones that observe daylight saving time.

### Label Reference

| Property   | Required | Default | Description                                  |
|------------|----------|---------|----------------------------------------------|
| `enabled`  | Yes      | —       | Enable/disable the job (`true`/`false`)      |
| `schedule` | Yes      | —       | Cron expression or `@every` interval         |
| `cmd`      | Yes      | —       | Command to execute inside the container      |
| `user`     | No       | container's `USER` | User to run the command as        |
| `timezone` | No       | Cronado's time zone (`TZ`, UTC in the image) | IANA time zone the schedule runs in (e.g., `Europe/Berlin`) |
| `timeout`  | No       | `30s`   | Max execution time (e.g., `30s`, `5m`, `1h`) |
| `jitter`   | No       | —       | Random delay of up to this duration before each run (e.g., `2m`); must be shorter than the schedule interval |
| `ping_url` | No       | —       | [Healthchecks.io](https://healthchecks.io)-compatible ping URL (see below) |
| `replicas` | No       | `once`  | For a Compose service with several replicas: run on one replica (`once`) or on each (`all`); see [Replicated services](#replicated-services) |

### Spreading start times (jitter)

Jobs with the same schedule start at the same second. With many containers on one host, for example several PHP apps running `0 * * * *`, that causes a load spike every hour and can hit external APIs or a shared database all at once. The `jitter` label delays each run by a random amount between zero and the given duration:

```
cronado.report.schedule=0 * * * *
cronado.report.jitter=5m
```

This job starts at a random time between :00 and :05 every hour. A new delay is picked for every run.

- **Allowed values:** any duration like `30s`, `5m` or `1h30m`. `0` or no label means no delay.
- **Must be shorter than the schedule interval.** `@every 10m` with `jitter=10m` is rejected, because a delayed run would collide with the next one. For schedules with uneven gaps, such as `0 9,10 * * *`, the shortest gap counts.
- **Status while waiting:** the job shows `pending` in the API until the delay is over, then `running`. A trigger that arrives while the job is still `pending` or `running` is skipped.
- **Next run:** `next_run` in the API and `cronado_job_next_run_timestamp_seconds` show the scheduled time without the delay. The actual start can be up to the jitter later.
- **Stops and restarts:** if cronado shuts down or the container stops during the wait, the run is dropped and not executed.
- **Timeout and pings:** the timeout starts when the command starts, not when the delay starts. Healthchecks pings are sent when the command starts, so allow for the jitter in the check's grace time.

### Replicated services

When a Compose service runs several replicas, for example with `docker compose up --scale web=3` or `deploy.replicas: 3`, every replica carries the same labels. By default, cronado runs each job of such a service only once per run, on one replica:

- **Which replica:** the one with the lowest Compose container number (`app-web-1` before `app-web-2`). Containers started with `docker compose run` only run the job if no regular replica of the service is running.
- **Takeover:** if that replica stops, the next one runs the job from the next scheduled run on. No run is repeated or made up.
- **Standby replicas:** the jobs of the other replicas stay registered and are shown with `"standby": true` in the API, `STANDBY` in `cronado cron-job list` and `cronado_job_standby 1`. They have no `next_run`. [Run now](#run-pause-and-resume-jobs) works on any replica.
- **Per replica:** set `replicas=all` for jobs that must run in every replica, for example clearing a local cache:

```yaml
services:
  web:
    image: my-app
    deploy:
      replicas: 3
    labels:
      cronado.cleanup.enabled: "true"
      cronado.cleanup.schedule: "0 3 * * *"
      cronado.cleanup.cmd: "php bin/console app:cleanup"       # once, on web-1
      cronado.cache.enabled: "true"
      cronado.cache.schedule: "*/15 * * * *"
      cronado.cache.cmd: "php bin/console cache:pool:prune"
      cronado.cache.replicas: "all"                             # on every replica
```

Replicas are matched by the Compose project, service and job name. Containers not created by Docker Compose are never treated as replicas. Cronado only sees the containers on its own Docker host, so replicas on other hosts each run the job.

### Checking labels before deployment

A mistake in a label, such as `0 25 * * *` or a typo like `timout`, only shows up in cronado's log once the container starts, and the job silently does not run. `cronado validate` checks the labels with the same rules the scheduler uses and lists every problem at once.

Check the services of a Compose file before you deploy it. No Docker access is needed, so this also works in CI:

```bash
cronado validate -f docker-compose.yml
cronado validate -f compose.yml -f compose.prod.yml   # merged like docker compose -f ... -f ...
```

Without `-f`, it checks all running containers on the host through the Docker socket, which shows what cronado sees right now:

```bash
docker exec cronado ./cronado validate
```

Example output:

```
app
  ✓ cleanup: @daily (enabled)
  ! cronado.cleanup.timout: unknown property "timout", valid properties are enabled,schedule,cmd,user,timezone,timeout,jitter,ping_url,replicas (label ignored)
  ✗ report: invalid cron schedule "0 25 * * *": end of range (25) above maximum (23): 25
db (container my-db)
  ✓ backup: 0 2 * * * (enabled)
  ✓ vacuum: @weekly (disabled)

Checked 2 containers: 3 jobs OK, 1 job rejected, 1 label ignored.
```

- **✓** the job is valid. Disabled jobs are listed too.
- **✗** the job is rejected and will not run. All reasons are listed, e.g. an invalid value and a missing `schedule`.
- **!** a single label is ignored, e.g. an unknown property or an invalid job name. The job itself may still run, but probably not as intended.
- **Exit code:** `0` if nothing was found, `1` if any job was rejected or label ignored, `2` if the Compose file or Docker could not be read. Use it in CI to fail a deployment.
- **Settings:** the label prefix, `CRONADO_MIN_SCHEDULE_INTERVAL` and `CRONADO_MAX_TIMEOUT` are read from the same config and environment as `cronado cron`. Run `validate` with the same settings as your scheduler, or limits like the minimum interval are not checked the same way. `docker exec cronado ./cronado validate` always uses the scheduler's own settings.
- **Compose limits:** only `labels` and `container_name` are read. Variables like `${VAR}` are not substituted; `$$` is read as `$`. Run `docker compose config > resolved.yml` first if your labels use variables.

### Dead man's switch pings

Failure notifications cannot tell you that a job stopped running altogether, for example because its container is gone. For that, give the job a `ping_url` from [Healthchecks.io](https://healthchecks.io) or a self-hosted Healthchecks instance:

```
cronado.backup.ping_url=https://hc-ping.com/5f2c1a3e-...
```

Cronado sends `POST <ping_url>/start` before each run, `POST <ping_url>` on success and `POST <ping_url>/fail` with the end of the output on failure. Healthchecks then alerts you when a ping is late or a run fails. Redirects are not followed, and the URL is redacted in the API unless an API token is set.

Ping URLs come from container labels, so by default they may only point to public addresses. For a self-hosted Healthchecks instance on a private network, set `CRONADO_PING_ALLOW_PRIVATE_NETWORKS=true`. Pings do not use `HTTP_PROXY`.

## Configuration

Cronado uses [Viper](https://github.com/spf13/viper) for configuration. When running in a container, use
environment variables — no config file is included in the image. When running from source, copy the template:

```bash
cp config.yaml.dist config.yaml
```

### Environment Variables

All settings can be set via environment variables using the `CRONADO_` prefix:

| Variable                                 | Description                          | Default           |
|------------------------------------------|--------------------------------------|-------------------|
| `CRONADO_LOG_LEVEL`                      | Log level (`debug`, `info`, `error`) | `info`            |
| `CRONADO_LOG_FORMAT`                     | Log format (`text`, `json`)          | `text`            |
| `CRONADO_CRON_LABEL_PREFIX`              | Label prefix for cron jobs           | `cronado`         |
| `CRONADO_SERVER_HOST`                    | HTTP server bind address             | `127.0.0.1`       |
| `CRONADO_SERVER_PORT`                    | HTTP server port                     | `8080`            |
| `CRONADO_SERVER_API_TOKEN`               | Bearer token for API/metrics auth    | — (disabled)      |
| `CRONADO_MIN_SCHEDULE_INTERVAL`          | Minimum interval between runs        | `1m`              |
| `CRONADO_MAX_TIMEOUT`                    | Maximum allowed job timeout          | `12h`             |
| `CRONADO_SHUTDOWN_TIMEOUT`               | How long to wait for running jobs when cronado stops; see [Stopping and updating cronado](#stopping-and-updating-cronado) | `8s` |
| `CRONADO_PING_ALLOW_PRIVATE_NETWORKS`    | Allow `ping_url` to reach private/loopback addresses | `false` |
| `CRONADO_DAEMON_WATCHER_ENABLED`         | Docker daemon health monitoring      | `true`            |
| `CRONADO_DAEMON_WATCHER_TIMEOUT_SECONDS` | Health check interval                | `5`               |
| `CRONADO_NOTIFY_INTERVAL_SECONDS`        | Notification throttle (same subject) | `3600`            |
| `CRONADO_NOTIFY_INCLUDE_OUTPUT`          | Add the end of the command output to failure notifications | `true` |
| `CRONADO_NOTIFY_EMAIL_ENABLED`           | Enable email notifications           | `false`           |
| `CRONADO_NOTIFY_EMAIL_SMTP_HOST`         | SMTP server host                     | —                 |
| `CRONADO_NOTIFY_EMAIL_SMTP_PORT`         | SMTP server port                     | `587`             |
| `CRONADO_NOTIFY_EMAIL_USERNAME`          | SMTP username                        | —                 |
| `CRONADO_NOTIFY_EMAIL_PASSWORD`          | SMTP password                        | —                 |
| `CRONADO_NOTIFY_EMAIL_FROM`              | Sender address                       | —                 |
| `CRONADO_NOTIFY_EMAIL_TO`                | Recipients (comma-separated)         | —                 |
| `CRONADO_NOTIFY_EMAIL_REQUIRE_TLS`       | Require TLS for SMTP connections     | `true`            |
| `CRONADO_NOTIFY_NTFY_ENABLED`            | Enable ntfy notifications            | `false`           |
| `CRONADO_NOTIFY_NTFY_SERVER`             | ntfy server URL                      | `https://ntfy.sh` |
| `CRONADO_NOTIFY_NTFY_TOPIC`              | ntfy topic                           | —                 |
| `CRONADO_NOTIFY_NTFY_TOKEN`              | ntfy auth token                      | —                 |
| `CRONADO_NOTIFY_WEBHOOK_ENABLED`         | Enable webhook notifications         | `false`           |
| `CRONADO_NOTIFY_WEBHOOK_URL`             | Webhook URL                          | —                 |
| `CRONADO_NOTIFY_WEBHOOK_FORMAT`          | `generic`, `slack`, `discord` or `gotify` | `generic`    |
| `CRONADO_METRICS_ENABLED`                | Enable Prometheus metrics            | `true`            |
| `CRONADO_METRICS_ENDPOINT`               | Metrics endpoint path                | `/metrics`        |

### Webhook Notifications

Webhooks send failure notifications to chat tools. Pick the payload format that matches the receiver:

| Format    | Payload                                   | Works with                          |
|-----------|-------------------------------------------|-------------------------------------|
| `generic` | `{"title": "...", "message": "..."}`      | Any JSON receiver (n8n, Home Assistant, ...) |
| `slack`   | `{"text": "..."}`                         | Slack, Mattermost, Rocket.Chat      |
| `discord` | `{"content": "..."}`                      | Discord                             |
| `gotify`  | `{"title": "...", "message": "...", "priority": 5}` | Gotify (`https://gotify.example.com/message?token=<app-token>`) |

```bash
CRONADO_NOTIFY_WEBHOOK_ENABLED=true
CRONADO_NOTIFY_WEBHOOK_URL=https://hooks.slack.com/services/...
CRONADO_NOTIFY_WEBHOOK_FORMAT=slack
```

Failure notifications include the last 20 lines of the command output (stderr, or stdout if stderr is empty). Set `CRONADO_NOTIFY_INCLUDE_OUTPUT=false` if your jobs print secrets.

## API

### Authentication

The API and metrics endpoints can optionally be protected with a Bearer token. Set the token via environment variable:

```bash
CRONADO_SERVER_API_TOKEN=my-secret-token
```

When a token is configured, all requests to `/api/*` and `/metrics` must include the `Authorization` header:

```bash
curl -H "Authorization: Bearer my-secret-token" http://127.0.0.1:8080/api/cron-job
```

When no token is set (the default), authentication is disabled and all read endpoints are open. The endpoints that change state ([run, pause and resume](#run-pause-and-resume-jobs)) are then disabled and return `403`, because anyone who can reach the port could otherwise run commands in your containers. Job commands are redacted in API responses, and Cronado logs a warning at startup if it listens on a non-loopback address. For network-exposed deployments, either set a token or place Cronado behind a reverse proxy with its own authentication.

### List Active Jobs

```bash
curl http://127.0.0.1:8080/api/cron-job
```

Response:

```json
[
  {
    "container_id": "abcdef123456",
    "cron_job": {
      "id": "abcdef123456-backup",
      "name": "backup",
      "enabled": true,
      "schedule": "0 2 * * *",
      "command": "pg_dump -U postgres mydb > /backups/daily.sql",
      "user": "postgres",
      "timeout": 600000000000,
      "status": "idle",
      "paused": false,
      "scheduler_id": 1,
      "last_run": "2026-10-13T02:00:00+02:00",
      "last_success": "2026-10-13T02:00:00+02:00",
      "last_result": "success",
      "last_duration_seconds": 12.4,
      "next_run": "2026-10-14T02:00:00+02:00"
    }
  }
]
```

Without an API token, `command` and `ping_url` are redacted. `timezone` is included only for jobs with a [`timezone`](#time-zones) label. `replicas` is `once` or `all`, and `standby` is `true` when another replica of the Compose service runs the job (see [Replicated services](#replicated-services)).

`id` identifies the job in the [run, pause and resume](#run-pause-and-resume-jobs) endpoints. `status` is `idle` between runs, `pending` while a run waits out its [jitter](#spreading-start-times-jitter) delay, and `running` while the command executes. `timeout` and `jitter` are durations in nanoseconds; `jitter` is omitted when not set.

### Run, Pause and Resume Jobs

These endpoints change state, so they **require `server.api_token`**. Without a token they return `403`.

```bash
# Run a job now, e.g. to test it after deployment
curl -X POST -H "Authorization: Bearer my-secret-token" http://127.0.0.1:8080/api/cron-job/abcdef123456-backup/run

# Skip the scheduled runs of a job, e.g. during maintenance
curl -X POST -H "Authorization: Bearer my-secret-token" http://127.0.0.1:8080/api/cron-job/abcdef123456-backup/pause
curl -X POST -H "Authorization: Bearer my-secret-token" http://127.0.0.1:8080/api/cron-job/abcdef123456-backup/resume
```

The job ID is the `id` field in `/api/cron-job`, or the `ID` column of `cronado cron-job list`. It is the short container ID plus the job name, so it changes when the container is recreated.

**Run** (`202 Accepted`):

- Starts the command immediately in the background and returns the job. The result appears in `last_result` once the command finishes.
- Skips the [jitter](#spreading-start-times-jitter) delay and also works while the job is paused. Everything else is the same as a scheduled run: user, timeout, metrics, failure notifications and pings.
- Returns `409 Conflict` if the job is already running or waiting out its jitter, so a job never runs twice at the same time.

**Pause and resume** (`200 OK`):

- A paused job skips its scheduled runs and logs `Skipping paused cron job` instead. It stays in the job list with `"paused": true` and without `next_run`.
- Pausing does not stop a run that is already in progress.
- Pausing and resuming are idempotent: pausing a paused job is not an error.
- **The pause is not saved.** A restart of the container or of cronado resumes the job, because its labels are read again. To disable a job for good, set `cronado.<job>.enabled=false`.

All three return `404` for an unknown job ID.

### List Label Problems

A job with invalid labels is not scheduled, so it does not appear in `/api/cron-job`. This endpoint lists those problems for all running containers, so a missing job is explained instead of silently absent:

```bash
curl http://127.0.0.1:8080/api/label-issues
```

Response:

```json
[
  {
    "container_id": "02c0a8bf4b6981903f9dff33db4f6ccda4ab7f218bc3cdc18e216f3bd2ddf630",
    "container_name": "my-app",
    "job": "report",
    "message": "invalid cron schedule \"0 25 * * *\": end of range (25) above maximum (23): 25",
    "job_rejected": true
  },
  {
    "container_id": "02c0a8bf4b6981903f9dff33db4f6ccda4ab7f218bc3cdc18e216f3bd2ddf630",
    "container_name": "my-app",
    "job": "cleanup",
    "label": "cronado.cleanup.timout",
    "message": "unknown property \"timout\", valid properties are enabled,schedule,cmd,user,timezone,timeout,jitter,ping_url,replicas",
    "job_rejected": false
  }
]
```

- **`job_rejected: true`:** the job is not scheduled. A job can have several entries, one per problem.
- **`job_rejected: false`:** only the named `label` is ignored. The job itself may still run, but probably not as intended.
- **Lifetime:** problems are recorded when a container starts and removed when it stops. Fixing the labels means recreating the container, which also clears its entries.
- The list is `[]` when there are no problems. Like `/api/cron-job`, it needs the API token if one is set.

`cronado validate` reports the same problems before deployment; see [Checking labels before deployment](#checking-labels-before-deployment).

### Health Check

`GET /healthz` needs no authentication. It returns `200` when cronado reaches the Docker daemon and is subscribed to its events, and `503` otherwise:

```json
{"status": "ok", "docker": "ok", "event_stream": "connected", "jobs": 3}
```

The Docker image runs `cronado healthcheck` as its `HEALTHCHECK`, which queries this endpoint.

### CLI

```bash
./cronado cron-job list                      # jobs registered in the running scheduler, plus label problems
./cronado cron-job run <job-id>              # run a job now (needs server.api_token)
./cronado cron-job pause <job-id>            # skip scheduled runs until resumed (needs server.api_token)
./cronado cron-job resume <job-id>
./cronado validate -f docker-compose.yml     # check labels before deployment
./cronado validate                           # check labels of running containers
```

See [Checking labels before deployment](#checking-labels-before-deployment) for `validate` and [Run, Pause and Resume Jobs](#run-pause-and-resume-jobs) for the job actions. The CLI talks to the running scheduler with the `server.*` settings and the API token from its config or environment, so inside the cronado container it works as is: `docker exec cronado ./cronado cron-job list`.

## Metrics

Prometheus metrics are available at `/metrics`:

| Metric                         | Type      | Description                                                     |
|--------------------------------|-----------|-----------------------------------------------------------------|
| `cronado_scheduled_jobs`       | Gauge     | Current number of scheduled jobs                                |
| `cronado_job_executions_total` | Counter   | Total executions (labels: `container_id`, `job_name`, `status`) |
| `cronado_job_duration_seconds` | Histogram | Execution duration (labels: `container_id`, `job_name`)         |
| `cronado_job_last_run_timestamp_seconds`     | Gauge | Unix time of the last run (labels: `container_id`, `container_name`, `job_name`) |
| `cronado_job_last_success_timestamp_seconds` | Gauge | Unix time of the last successful run (same labels)    |
| `cronado_job_next_run_timestamp_seconds`     | Gauge | Unix time of the next scheduled run (same labels)     |
| `cronado_job_running`                        | Gauge | `1` while the job runs, `0` otherwise (same labels)   |
| `cronado_job_paused`                         | Gauge | `1` while the job is paused, `0` otherwise (same labels) |
| `cronado_job_standby`                        | Gauge | `1` while another replica of the Compose service runs the job, `0` otherwise (same labels); see [Replicated services](#replicated-services) |
| `cronado_label_issues`                       | Gauge | Number of label problems of a running container (labels: `container_id`, `container_name`, `job_name`, `kind` = `rejected_job` or `ignored_label`); see [List Label Problems](#list-label-problems) |

Alert when a job has not succeeded for a day:

```yaml
- alert: CronadoJobNotSucceeding
  expr: time() - cronado_job_last_success_timestamp_seconds > 86400
```

Alert when a running container has invalid cron job labels:

```yaml
- alert: CronadoInvalidLabels
  expr: sum by (container_name, job_name) (cronado_label_issues) > 0
```

Alert when a job was paused and then forgotten:

```yaml
- alert: CronadoJobPausedTooLong
  expr: cronado_job_paused == 1
  for: 24h
```

Prometheus scrape config:

```yaml
scrape_configs:
  - job_name: "cronado"
    static_configs:
      - targets: [ "127.0.0.1:8080" ]
```

## Advanced Setup

### Stopping and updating cronado

When cronado stops, for example on `docker stop` or a Watchtower update, it:

1. Stops starting new runs. Runs still waiting out their [jitter](#spreading-start-times-jitter) are dropped, and manual runs are refused with `503`.
2. Waits for running jobs to finish, for at most `shutdown_timeout` (default `8s`). Their results, notifications and pings are recorded as usual.
3. Exits. Jobs still running at that point are logged as `Shutdown timeout reached, abandoning running cron job`.

An abandoned command is **not** killed. It keeps running inside its container until it finishes or reaches its own `timeout`, but cronado does not record its result, send a notification or send the final [ping](#dead-mans-switch-pings). With a `ping_url`, Healthchecks reports the run as started but never finished, so it does not go unnoticed.

Docker kills a container that has not stopped after its stop grace period, 10 seconds by default. The default `shutdown_timeout` of `8s` fits inside that. If your jobs run longer and should finish before an update, raise both, keeping the grace period a few seconds above the timeout:

```yaml
services:
  cronado:
    image: ghcr.io/teqneers/cronado:latest
    stop_grace_period: 5m
    environment:
      CRONADO_SHUTDOWN_TIMEOUT: "4m50s"
```

`shutdown_timeout: "0s"` waits for running jobs without a limit, which only makes sense outside Docker or with an equally unlimited grace period.

### Docker Socket Proxy

Mounting `/var/run/docker.sock` directly gives cronado (and any attacker who compromises it) full control over the Docker daemon. For production environments, it is strongly recommended to place a socket proxy in between so that only the API endpoints cronado actually needs are exposed.

The example below uses [wollomatic/socket-proxy](https://github.com/wollomatic/socket-proxy). The `-allowGET` and `-allowPOST` flags are scoped to exactly what cronado requires: reading container/event info and creating exec sessions.

```yaml
services:
  cronado:
    image: ghcr.io/teqneers/cronado:latest
    container_name: cronado
    restart: unless-stopped
    depends_on:
      docker-proxy:
        condition: service_healthy
    environment:
      CRONADO_SERVER_HOST: "0.0.0.0"
      DOCKER_HOST: "tcp://docker-proxy:2375"
    ports:
      - "8080:8080"
    networks:
      - cronado
      - docker-proxy

  docker-proxy:
    image: wollomatic/socket-proxy:1
    container_name: cronado-docker-proxy
    restart: unless-stopped
    read_only: true
    user: "65534:988"   # nobody:docker — adjust GID to match your host's docker group
    cap_drop:
      - ALL
    security_opt:
      - no-new-privileges
    command:
      - '-loglevel=info'
      - '-allowhealthcheck'
      - '-allowfrom=cronado:cronado'
      - '-listenip=0.0.0.0'
      - '-allowGET=/(v[.0-9]*/)?(containers/.*|exec/.*|info|events)'
      - '-allowPOST=/(v[.0-9]*/)?(containers|exec)/.*'
      - '-allowHEAD=.*'
      - '-watchdoginterval=30'
      - '-stoponwatchdog'
      - '-shutdowngracetime=5'
    healthcheck:
      test: ["CMD", "./healthcheck"]
      interval: 10s
      timeout: 5s
      retries: 2
    networks:
      - docker-proxy
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock

networks:
  cronado:
  docker-proxy:
```

> **Note:** The `-allowfrom` flag restricts which containers may connect to the proxy. Set it to the cronado
> container name or network CIDR. The `user` GID (`988` above) must match the `docker` group on your host
> (`getent group docker` to check).

## Troubleshooting

### Docker socket permission denied

Ensure the cronado container has access to the Docker socket:

```bash
docker run -v /var/run/docker.sock:/var/run/docker.sock:ro ...
```

If running as a non-root user on the host, you may need to add the user to the `docker` group.

### Container not being detected

- Verify labels follow the correct format: `cronado.<job-name>.<property>`
- Check that `enabled` is set to `true`
- Ensure `schedule` and `cmd` labels are present
- Run `cronado cron-job list` or `curl http://127.0.0.1:8080/api/label-issues` to see the label problems cronado found
- Run `cronado validate` to check labels before deploying, or the labels of all running containers
- Run with `CRONADO_LOG_LEVEL=debug` for detailed output

### Commands timing out

The default timeout is 30 seconds. A command that exceeds its timeout is killed inside the container. For long-running commands, set a per-job timeout:

```
cronado.my-job.timeout=10m
```

### Permission denied after upgrading to 1.1

Since 1.1.0, commands run as the container's own `USER` instead of `root` when no `user` label is set. If a job needs root, set it explicitly:

```
cronado.my-job.user=root
```


## Security

Cronado includes several built-in security measures:

- **API authentication** -- optional Bearer token protects `/api/*` and `/metrics` endpoints
- **TLS-enforced email** -- SMTP notifications use mandatory TLS by default (configurable via `CRONADO_NOTIFY_EMAIL_REQUIRE_TLS`)
- **Least privilege by default** -- commands run as the container's own `USER` unless a `user` label says otherwise
- **Command redaction in the API** -- without an API token, job commands are not returned by `/api/cron-job`
- **Schedule rate limiting** -- `@every` and seconds-based schedules are rejected if they run more often than the minimum interval (default 1 minute), preventing accidental DoS
- **Enforced timeouts** -- commands that exceed their timeout are killed inside the container, and captured output is capped at 64 KiB
- **Timeout cap** -- job timeouts are capped at a configurable maximum (default 12 hours) to prevent runaway executions
- **Job name validation** -- only alphanumeric characters, hyphens, and underscores are accepted in job names
- **Command log redaction** -- commands are truncated in INFO logs to avoid leaking credentials; full commands are logged at DEBUG level only

For a full discussion of the security model, including Docker socket access and `sh -c` execution, see [SECURITY.md](SECURITY.md).

## Contributors

Special gratitude to **Sven Walloner** — original author and creator of
Cronado.

- **Oliver Mueller**

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Security

For security concerns, see [SECURITY.md](SECURITY.md).

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.

Copyright (c) 2026 [TEQneers GmbH & Co. KG](https://teqneers.de)

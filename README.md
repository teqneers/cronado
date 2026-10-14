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
- HTTP API to list active cron jobs with last result and next run
- CLI for management (`cron` to start, `cron-job list` to inspect, `healthcheck`)
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
cronado.<job-name>.ping_url=https://hc-ping.com/<uuid>
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
| `ping_url` | No       | —       | [Healthchecks.io](https://healthchecks.io)-compatible ping URL (see below) |

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

When no token is set (the default), authentication is disabled and all endpoints are open. Job commands are then redacted in API responses, and Cronado logs a warning at startup if it listens on a non-loopback address. For network-exposed deployments, either set a token or place Cronado behind a reverse proxy with its own authentication.

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
      "name": "backup",
      "enabled": true,
      "schedule": "0 2 * * *",
      "command": "pg_dump -U postgres mydb > /backups/daily.sql",
      "user": "postgres",
      "timeout": 600000000000,
      "status": "idle",
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

Without an API token, `command` and `ping_url` are redacted. `timezone` is included only for jobs with a [`timezone`](#time-zones) label.

### Health Check

`GET /healthz` needs no authentication. It returns `200` when cronado reaches the Docker daemon and is subscribed to its events, and `503` otherwise:

```json
{"status": "ok", "docker": "ok", "event_stream": "connected", "jobs": 3}
```

The Docker image runs `cronado healthcheck` as its `HEALTHCHECK`, which queries this endpoint.

### CLI

```bash
./cronado cron-job list
```

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

Alert when a job has not succeeded for a day:

```yaml
- alert: CronadoJobNotSucceeding
  expr: time() - cronado_job_last_success_timestamp_seconds > 86400
```

Prometheus scrape config:

```yaml
scrape_configs:
  - job_name: "cronado"
    static_configs:
      - targets: [ "127.0.0.1:8080" ]
```

## Advanced Setup

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

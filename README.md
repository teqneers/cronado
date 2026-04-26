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
- HTTP API to list active cron jobs
- CLI for management (`cron` to start, `cron-job list` to inspect)
- Prometheus metrics (`/metrics`)
- Failure notifications via email (SMTP) and [ntfy.sh](https://ntfy.sh)
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
cronado.<job-name>.timeout=5m
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
| 6-field cron  | `0 30 * * * *` | Every hour at :30      |
| `@every`      | `@every 5m`    | Every 5 minutes        |
| `@hourly`     | `@hourly`      | Every hour             |
| `@daily`      | `@daily`       | Once a day at midnight |
| `@weekly`     | `@weekly`      | Once a week            |
| `@monthly`    | `@monthly`     | Once a month           |

### Label Reference

| Property   | Required | Default | Description                                  |
|------------|----------|---------|----------------------------------------------|
| `enabled`  | Yes      | —       | Enable/disable the job (`true`/`false`)      |
| `schedule` | Yes      | —       | Cron expression or `@every` interval         |
| `cmd`      | Yes      | —       | Command to execute inside the container      |
| `user`     | No       | `root`  | User to run the command as                   |
| `timeout`  | No       | `30s`   | Max execution time (e.g., `30s`, `5m`, `1h`) |

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
| `CRONADO_MIN_SCHEDULE_INTERVAL`          | Minimum `@every` interval            | `1m`              |
| `CRONADO_MAX_TIMEOUT`                    | Maximum allowed job timeout          | `12h`             |
| `CRONADO_DAEMON_WATCHER_ENABLED`         | Docker daemon health monitoring      | `true`            |
| `CRONADO_DAEMON_WATCHER_TIMEOUT_SECONDS` | Health check interval                | `5`               |
| `CRONADO_NOTIFY_INTERVAL_SECONDS`        | Notification throttle (same subject) | `3600`            |
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
| `CRONADO_METRICS_ENABLED`                | Enable Prometheus metrics            | `true`            |
| `CRONADO_METRICS_ENDPOINT`               | Metrics endpoint path                | `/metrics`        |

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

When no token is set (the default), authentication is disabled and all endpoints are open. For network-exposed deployments, either set a token or place Cronado behind a reverse proxy with its own authentication.

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
      "scheduler_id": 1
    }
  }
]
```

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

The default timeout is 30 seconds. For long-running commands, set a per-job timeout:

```
cronado.my-job.timeout=10m
```

### Default root user

Commands run as `root` by default. Set the `user` label explicitly to use least privilege:

```
cronado.my-job.user=nobody
```


## Security

Cronado includes several built-in security measures:

- **API authentication** -- optional Bearer token protects `/api/*` and `/metrics` endpoints
- **TLS-enforced email** -- SMTP notifications use mandatory TLS by default (configurable via `CRONADO_NOTIFY_EMAIL_REQUIRE_TLS`)
- **Schedule rate limiting** -- `@every` schedules are rejected if below the minimum interval (default 1 minute), preventing accidental DoS
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

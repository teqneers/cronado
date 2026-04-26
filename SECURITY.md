# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability, please [open a GitHub issue](https://github.com/teqneers/cronado/issues/new) with the label `security`.

Please include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact

## Security Scope

### By Design

Cronado requires access to the **Docker socket** (`/var/run/docker.sock`) to function. This is a privileged operation. Users should:

- Mount the socket as **read-only** where possible (`/var/run/docker.sock:/var/run/docker.sock:ro`)
- Consider using a [Docker socket proxy](https://github.com/Tecnativa/docker-socket-proxy) to limit API access
- Run Cronado in a dedicated, minimal container

### Command Execution

Commands defined in container labels are executed via `docker exec` using `sh -c`. This is intentional — Cronado is designed to run arbitrary commands defined by the Docker host administrator. The security boundary is **who can set container labels**, which is controlled by Docker access permissions.

### Default User

Commands execute as `root` by default if no `user` label is specified. A warning is logged when this happens. Always set the `user` label explicitly to run commands with least privilege:

```
cronado.my-job.user=nobody
```

### Credentials

SMTP and ntfy credentials should be passed via **environment variables** (e.g., `CRONADO_NOTIFY_EMAIL_PASSWORD`) rather than config files, especially in shared environments.

## Security Features

### API Authentication

The HTTP API (`/api/cron-job`) and metrics endpoint (`/metrics`) can be protected with Bearer token authentication. This is optional but recommended when the API is exposed beyond localhost.

**Configuration:**

```bash
# Via environment variable (recommended)
export CRONADO_SERVER_API_TOKEN="your-secret-token"

# Or in config.yaml
server:
  api_token: "your-secret-token"
```

**Usage:**

```bash
curl -H "Authorization: Bearer your-secret-token" http://localhost:8080/api/cron-job
```

When `api_token` is empty (default), authentication is disabled and all requests pass through. This preserves backwards compatibility.

For production deployments, you can also place a reverse proxy (e.g., nginx, Traefik) with its own authentication in front of Cronado.

### Email TLS Enforcement

SMTP connections require TLS by default (`require_tls: true`). This prevents credentials from being transmitted in plaintext. To disable TLS enforcement (e.g., for local test SMTP servers), set:

```bash
export CRONADO_NOTIFY_EMAIL_REQUIRE_TLS=false
```

### Schedule Rate Limiting

The `min_schedule_interval` setting (default `1m`) prevents excessively frequent `@every` schedules that could cause resource exhaustion. To override:

```bash
export CRONADO_MIN_SCHEDULE_INTERVAL="10s"
```

Standard cron expressions (5-field) have a natural minimum of 1 minute.

### Timeout Limits

The `max_timeout` setting (default `12h`) caps the maximum allowed job execution timeout. This prevents jobs from holding resources indefinitely. To override:

```bash
export CRONADO_MAX_TIMEOUT="24h"
```

### Job Name Validation

Job names extracted from container labels must match the pattern `[a-zA-Z0-9][a-zA-Z0-9_-]*`. Labels with invalid job names are ignored and a warning is logged.

## Supported Versions

We provide security fixes for the latest release only.

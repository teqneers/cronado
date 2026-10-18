# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability, please report it privately via [GitHub Security Advisories](https://github.com/teqneers/cronado/security/advisories/new). Please do not open a public issue.

Please include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact

## Security Scope

### By Design

Cronado requires access to the **Docker socket** (`/var/run/docker.sock`) to function. This is a privileged operation. Users should:

- Treat Cronado as **root-equivalent on the host**. Mounting the socket read-only (`:ro`) does not restrict the Docker API; it only prevents replacing the socket file.
- A [Docker socket proxy](https://github.com/Tecnativa/docker-socket-proxy) gives limited protection, because Cronado needs the `CONTAINERS`, `EXEC` and `POST` permissions
- Run Cronado in a dedicated, minimal container

### Command Execution

Commands defined in container labels are executed via `docker exec` using `sh -c`. This is intentional — Cronado is designed to run arbitrary commands defined by the Docker host administrator. The security boundary is **who can set container labels**, which is controlled by Docker access permissions.

Jobs with an `image` make Cronado create a new container with the given volumes, network and environment, including bind mounts of host paths. Docker copies the labels of an image to its containers, so an untrusted image could define such a job and get access to the host. Therefore `image` labels are rejected unless `allow_label_image_jobs` is set; jobs with an `image` in the config file, which only the operator controls, always work. For these jobs, a socket proxy must also allow image pulls and creating and removing containers.

### Default User

Commands run as the **container's own configured user** (its `USER`) when no `user` label is specified. To run a command as a different user, for example root, set it explicitly:

```
cronado.my-job.user=root
```

### Timeouts

A command that exceeds its timeout is killed inside the container with `timeout -s KILL` (available in busybox and coreutils images). Command output kept by Cronado is capped at 64 KiB per stream.

### Command Output in Notifications

Failure notifications include the last 20 lines of the command output, and `ping_url` failure pings the last 10 KiB. If a job may print secrets, set `CRONADO_NOTIFY_INCLUDE_OUTPUT=false` and do not give it a `ping_url`.

### Outgoing Requests from Labels

The `ping_url` label lets whoever sets container labels make Cronado send HTTP(S) POST requests. To prevent this from reaching services on Cronado's own network, such as a Docker socket proxy or a cloud metadata endpoint:

- Only `http` and `https` URLs are accepted, redirects are not followed and no proxy is used.
- By default, the resolved address must be public. All IANA special-purpose ranges are refused at connect time: loopback, private (RFC 1918, RFC 4193), link-local, CGNAT, reserved, documentation and multicast, plus IPv6 transition ranges that can embed an internal IPv4 address (IPv4-mapped, IPv4-compatible, NAT64, 6to4, Teredo). The check runs on the resolved IP, so DNS tricks cannot bypass it.
- `ping.allow_private_networks` (`CRONADO_PING_ALLOW_PRIVATE_NETWORKS`) lifts this restriction for self-hosted Healthchecks. Enable it only if label authors are trusted.

Ping URLs, webhook URLs and ntfy URLs are never written to logs, because they usually embed a secret.

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

When `api_token` is empty (default), authentication is disabled and all requests pass through. In that case job commands are **redacted** in API responses, because they often contain credentials, and Cronado logs a warning at startup if the server listens on a non-loopback address.

For production deployments, you can also place a reverse proxy (e.g., nginx, Traefik) with its own authentication in front of Cronado.

### Email TLS Enforcement

SMTP connections require TLS by default (`require_tls: true`). This prevents credentials from being transmitted in plaintext. To disable TLS enforcement (e.g., for local test SMTP servers), set:

```bash
export CRONADO_NOTIFY_EMAIL_REQUIRE_TLS=false
```

### Schedule Rate Limiting

The `min_schedule_interval` setting (default `1m`) prevents excessively frequent schedules that could cause resource exhaustion. It applies to `@every` schedules and to 6-field expressions with a seconds field. To override:

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

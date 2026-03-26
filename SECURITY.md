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

Commands execute as `root` by default if no `user` label is specified. Always set the `user` label explicitly to run commands with least privilege:

```
cronado.my-job.user=nobody
```

### Credentials

SMTP and ntfy credentials should be passed via **environment variables** (e.g., `CRONADO_NOTIFY_EMAIL_PASSWORD`) rather than config files, especially in shared environments.

## Supported Versions

We provide security fixes for the latest release only.

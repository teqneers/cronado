# Contributing to Cronado

Thank you for your interest in contributing to Cronado! Here's how to get started.

## Development Setup

### Prerequisites

- [Go 1.24+](https://go.dev/dl/)
- [Docker](https://docs.docker.com/get-docker/) (for integration testing)
- [golangci-lint](https://golangci-lint.run/welcome/install-locally/) (for linting)

### Build

```bash
git clone https://github.com/teqneers/cronado.git
cd cronado
go build -o cronado main.go
```

### Run Tests

```bash
go test ./...
```

### Lint

```bash
golangci-lint run
```

## Making Changes

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Make your changes
4. Ensure tests pass and linter is clean
5. Commit with a descriptive message
6. Push to your fork and open a Pull Request

## Code Style

- Follow standard Go conventions (`gofmt`, `goimports`)
- Use meaningful variable and function names
- Add tests for new functionality
- Keep changes focused — one feature or fix per PR

## Reporting Issues

Use the [issue templates](https://github.com/teqneers/cronado/issues/new/choose) to report bugs or request features. Include as much detail as possible, especially log output with `CRONADO_LOG_LEVEL=debug`.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).

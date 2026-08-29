# Contributing to AGIS

Thanks for your interest in contributing to AGIS! We welcome contributions, bug reports, feature suggestions, and pull requests from the community.

## How to Contribute

1. **Fork the repository** and create your feature branch from `main`.
2. **Open an issue first** for substantial features or architectural changes — describe what you plan to build so we can align on design early.
3. **Run tests & verification** before submitting your pull request:
   ```bash
   # build the binary
   go build ./cmd/agis

   # run tests with race detector
   go test -race ./...

   # format and vet code
   go vet ./...
   gofmt -s -w .
   ```
4. **Submit a pull request** with a clear description of the problem solved, changes made, and test evidence.

## Guidelines & Architecture

- **Go 1.26+**: Ensure your code is compatible with Go 1.26+ and formatted with standard Go conventions.
- **Zero external runtime services**: Preserve the single static binary philosophy (pure Go SQLite, no mandatory CGO, no heavy external daemon dependencies).
- **Hexagonal Architecture**: Maintain strict separation of concerns. The domain in `internal/core` must never import an adapter (`internal/memory`, `internal/adapters`, `internal/gateway`, etc.); adapters plug in behind domain ports.
- **Security & Permissions**: Background daemons and external interfaces must respect `PolicyGuard` with fail-closed postures (`sandbox` by default with `AutoDenyApprover`).
- **Tests**: Write unit and integration tests for new features. Use `go.uber.org/goleak` for concurrent workers and background daemons.

## Security

Report security vulnerabilities privately. Do **not** open a public issue for zero-days or sensitive security bugs. Contact the maintainer directly or use GitHub Security Advisories.

## License

By contributing, you agree that your contributions will be licensed under the project's [MIT License](LICENSE).

# Changelog

All notable changes to this project will be documented in this file.

## v0.1.0 - 2026-02-07

### Added
- Simple `Request` helper with `Perform(...)` wrapper over `net/http`.
- Body encoding for JSON, XML, and plain text.
- Optional retry logic with exponential backoff for idempotent methods.
- Custom `http.Client` injection support.
- Basic documentation and examples.
- CI workflow running `go fmt`, `go vet`, `go test`, and `golangci-lint`.
- Test coverage for retry behavior and client injection.

# AGENTS.md

This repository is a standalone example application for the Smile ID Go SDK.

## Development rules

- Keep the application independent of Smile ID internals. Use only the public SDK API exposed by `github.com/smileidentity/smileid-sdk-go/v12`.
- Preserve the testbench behavior: commands should be testable against a local `httptest` server without real Smile ID credentials.
- Prefer small, explicit examples over broad framework abstractions.
- Keep credentials out of source control and documentation examples.
- Run `go test ./...` before handing off changes.
- Run `go vet ./...` for changes touching command execution, HTTP, TLS, or error handling.

## Layout

- `cmd/smileid-example-go` contains the executable entrypoint.
- `internal/example` contains command parsing, SDK configuration, command handlers, and integration-style tests.
- `.github/workflows/ci.yml` runs Go tests, vet, and Semgrep.
- `.github/dependabot.yml` keeps Go modules and GitHub Actions current.

## Local SDK development

`go.mod` intentionally uses:

```go
replace github.com/smileidentity/smileid-sdk-go/v12 => ..
```

Keep this while the example lives inside the SDK checkout. Remove it only when publishing the example independently from the SDK repository.

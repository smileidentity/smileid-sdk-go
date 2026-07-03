# AGENTS.md

This repository holds Smile ID's server-side SDK for Go, covering the V3 APIs.

## Source of truth

The API surface — endpoints, request shapes, response shapes — comes from the OpenAPI specifications published at https://github.com/smileidentity/api-reference. Treat that repository as authoritative. Do not hand-write request or response models that duplicate what the specs already define.

## Layout

- `generated/models/` holds wire request and response models. It's owned by the generation pipeline — don't hand-edit it, regenerate it instead.
- `generated/operations/` holds one thin function per operation, mapping typed parameters to transport requests. Also generator-owned.
- The root package (`client.go`, `transport.go`, `auth.go`, `errors.go`, `helpers.go`, and the resource files) is hand-written and wraps the generated layer. It must survive regeneration.

## Running tests

Build, test, and lint with:

```bash
go build ./...
go test ./...
gofmt -l .
go vet ./...
golangci-lint run
```

The end-to-end test in `e2e_test.go` skips unless `SMILE_PARTNER_ID` and `SMILE_API_KEY` are set in the environment.

## Org-wide agent conventions

Internal contributors should also read the shared agent conventions at https://github.com/smileidentity/agents (a private repository).

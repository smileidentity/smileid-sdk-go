# AGENTS.md

This repository holds Smile ID's server-side SDK for Go, covering the V3 APIs.

## Source of truth

The API surface (endpoints, request and response shapes) comes from `specs/v3/openapi.yaml` in [smileidentity/api-reference](https://github.com/smileidentity/api-reference). Read it at a release tag such as `v3.1115.0`, not at `main`. When a task or pull request names a tag, use that one. When the SDK and the spec disagree, the spec wins.

- Ignore anything marked `x-smile-sdk-exclude: true` or `x-internal: true`. These are server-to-server details, such as the HMAC signature headers, that the SDKs leave out on purpose. The SDK doesn't sign requests. It authenticates every call with the spec's `SmileIDToken` scheme: it gets a token from `POST /v3/token` and sends it in the `SmileID-Token` header, in `transport.go`. Keep that code when you ignore the excluded headers.
- Don't add endpoints or fields that aren't in the spec. Internal endpoints are kept out of it on purpose.

## Layout

- `generated/models/` holds wire request and response models, and `generated/operations/` holds one thin function per operation, mapping typed parameters to transport requests. Both are hand-written for now, because there is no generator yet. Edit it by hand, but keep it shaped like generator output: wire models and one thin function per operation, with no business logic. A generator will replace it later.
- The root package (`client.go`, `transport.go`, `errors.go`, `helpers.go`, and the resource files) is hand-written and wraps the generated layer. It must survive the switch to generated code.

## Keeping the SDKs in step

This SDK is one of the `smileid-sdk-*` repositories, which share one design. A change to the public API here should be made the same way in the others: the same operations, option names and defaults, adjusted only for each language's naming conventions.

Automated pull requests use these labels and branches:

| Label | Branch | Meaning |
|---|---|---|
| `spec-sync` | `spec-sync/rolling` | Brings the SDK up to a new spec tag. There is one rolling pull request at a time, and the automation finds it by this branch name, so don't open another. |
| `spec-sync` | `spec-sync/<scope>` | A run limited to one product, such as the backlog run split by product. |
| `spec-sync` and `breaking` | `spec-sync/breaking-<tag>` | A change that needs a major release. Breaking changes always get their own pull request. |
| `release` | `release/vX.Y.Z` | A version bump and changelog entry. Made by hand for now. An automated release train is planned. |

Porting changes between SDKs automatically, with the `parity-port` label, is planned but not built yet.

The `breaking` label is not only for automation. CI fails any pull request that breaks the public API unless it has the `breaking` label, including pull requests made by hand.

Add `parity:skip` to a pull request that should not be ported to the other SDKs.

Tests:

- Every new operation needs a golden fixture test, which checks the exact request body and headers, and tests that parse its responses. Golden fixture tests live in `serialization_test.go`; follow the existing cases there. Shared persona cases for integration tests live in the private `smileid-sdk-fixtures` repository.
- Changed lines must be fully covered by tests. Don't add coverage exclusion comments.

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

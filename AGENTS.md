# AGENTS.md

This repository holds Smile ID's server-side SDK for Go, covering the V3 APIs.

## Source of truth

The API surface (endpoints, request and response shapes) comes from `specs/v3/openapi.yaml` in [smileidentity/api-reference](https://github.com/smileidentity/api-reference). Read it at a release tag such as `v3.1115.0`, not at `main`. When a task or pull request names a tag, use that one. When the SDK and the spec disagree, the spec wins.

- Ignore anything marked `x-smile-sdk-exclude: true`. These are server-to-server details, such as the HMAC signature headers, that the SDKs leave out on purpose.
- Don't add endpoints or fields that aren't in the spec. Internal endpoints are kept out of it on purpose.

## Layout

- `generated/models/` holds wire request and response models, and `generated/operations/` holds one thin function per operation, mapping typed parameters to transport requests. Both are hand-written for now, because there is no generator yet. Edit it by hand, but keep it shaped like generator output: wire models and one thin function per operation, with no business logic. A generator will replace it later.
- The root package (`client.go`, `transport.go`, `auth.go`, `errors.go`, `helpers.go`, and the resource files) is hand-written and wraps the generated layer. It must survive the switch to generated code.

## Keeping the SDKs in step

This SDK is one of the `smileid-sdk-*` repositories, which share one design. A change to the public API here should be made the same way in the others: the same operations, option names and defaults, adjusted only for each language's naming conventions.

Automated pull requests use these labels and branches:

| Label | Branch | Meaning |
|---|---|---|
| `spec-sync` | `bot/spec-sync` | Brings the SDK up to a new spec tag. One rolling pull request at a time. |
| `breaking` | `bot/spec-sync-breaking-<tag>` | A change that needs a major release. Breaking changes always get their own pull request. |
| `parity-port` | `bot/parity-port-<repo>-<number>` | A change ported from another SDK's merged pull request, which it links to. |
| `release` | `bot/release-v<version>` | A version bump and changelog entry. |

Add `parity:skip` to a pull request that should not be ported to the other SDKs.

Tests:

- Every new operation needs a golden fixture test, which checks the exact request body and headers, and tests that parse its responses.
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

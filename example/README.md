# Smile ID Go SDK Example

This repository is a small command-line application that demonstrates how an external developer can use the Smile ID Go SDK without depending on Smile ID internals.

It is also a testbench: the test suite runs the same application code against a local TLS fake Smile ID API and verifies the SDK sends the expected requests.

## What it demonstrates

- SDK client construction from environment variables or flags.
- Unauthenticated service lookups.
- Authenticated token retrieval through the SDK.
- Enhanced KYC submission with user details and consent.
- Verification status retrieval.
- Callback replay.
- Typed, JSON-friendly outputs that are easy to script.

## Requirements

- Go 1.21 or later.
- Smile ID sandbox credentials for real API calls.

## Setup

The module currently uses a local `replace` directive:

```go
replace github.com/smileidentity/smileid-sdk-go/v12 => ..
```

That keeps this example pinned to the parent SDK checkout during SDK development. If you copy this example elsewhere, remove the `replace` line and run:

```bash
go get github.com/smileidentity/smileid-sdk-go/v12
```

## Configuration

Set credentials with environment variables:

```bash
export SMILE_PARTNER_ID="12345"
export SMILE_API_KEY="..."
export SMILE_CALLBACK_URL="https://your-app.example.com/smile-callback"
```

Optional variables:

- `SMILE_PARTNER_SECRET` enables optional HMAC request signing.
- `SMILE_BASE_URL` overrides the SDK environment URL.
- `SMILE_TIMEOUT` sets the per-request timeout, for example `30s`.

The same values can be passed as global flags:

```bash
go run ./cmd/smileid-example-go --partner-id 12345 --api-key "$SMILE_API_KEY" services
```

## Commands

List reference data:

```bash
go run ./cmd/smileid-example-go services --country NG
```

Submit an Enhanced KYC job:

```bash
go run ./cmd/smileid-example-go \
  enhanced-kyc \
  --country NG \
  --id-type NIN \
  --id-number 12345678901 \
  --given-names Amina \
  --last-name Okafor \
  --email amina@example.com \
  --privacy-url https://your-app.example.com/privacy
```

Retrieve status:

```bash
go run ./cmd/smileid-example-go status --job-id job_...
```

Replay a callback:

```bash
go run ./cmd/smileid-example-go replay \
  --job-id job_... \
  --callback-url https://your-app.example.com/smile-callback
```

## Development

Run the testbench:

```bash
go test ./...
```

Run static checks:

```bash
go vet ./...
```

Run Semgrep if installed:

```bash
semgrep scan --config .semgrep.yml
```

The tests use `SMILE_EXAMPLE_INSECURE_TLS=1` only for the in-process `httptest` TLS server. Do not use that setting for real Smile ID API calls.

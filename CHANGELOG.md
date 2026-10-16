# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [12.2.0] - 2026-10-08

### Changed

- Each request mints its own token, bound to that request's `user_id` and product, instead of reusing one cached token. A 401 is no longer retried with a refreshed token.
- A `UserID` that is empty or padded with whitespace returns a `*ValidationError` before any request is sent.
- `Biometric.Authenticate` requires `SelfieImage` and `LivenessImages`.

### Added

- `usesmileid.WithEnrolledImage()` verifies a biometric KYC job against the user's enrolled selfie; the grant is bound into the job's token and the selfie and liveness images may be omitted. `UserID` is then required.

### Removed

- `UseEnrolledImage` from `AuthenticationParams`; selfie and liveness images are now always required.

## [12.1.0] - 2026-10-01

### Added

- Residency document verification: `client.Documents.VerifyResidency`. `IDType` defaults to `PASSPORT`.

## [12.0.0] - 2026-08-20

First public release.

### Added

- Enhanced KYC, biometric KYC, document verification and enhanced document
  verification.
- Biometric enrollment, authentication and compare.
- Verification status retrieval, with a `WaitUntilComplete` helper that polls
  until a job reaches a decision.
- Callback replay for a job.
- Fraud reporting, with `FlagFraud` and `ClearFraud` wrappers.
- Bank codes, supported ID types, supported documents and ID status lookups.
- Sandbox and production environments, with a `BaseURL` override for any
  other host.
- Internal JWT authentication, cached and refreshed automatically.
- A typed error hierarchy, matchable with `errors.As`.
- Automatic retries with exponential backoff for idempotent calls.
- Zero runtime dependencies beyond the Go standard library.

[Unreleased]: https://github.com/smileidentity/smileid-sdk-go/compare/v12.2.0...HEAD
[12.2.0]: https://github.com/smileidentity/smileid-sdk-go/compare/v12.1.0...v12.2.0
[12.1.0]: https://github.com/smileidentity/smileid-sdk-go/compare/v12.0.0...v12.1.0
[12.0.0]: https://github.com/smileidentity/smileid-sdk-go/releases/tag/v12.0.0

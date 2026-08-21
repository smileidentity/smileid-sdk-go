# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/smileidentity/smileid-sdk-go/compare/v12.0.0...HEAD
[12.0.0]: https://github.com/smileidentity/smileid-sdk-go/releases/tag/v12.0.0

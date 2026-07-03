# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- `BaseURL` must be an absolute https URL with no query or fragment, and
  `DefaultCallbackURL` and per-request callback URLs must be absolute https
  URLs. Insecure values fail with a local validation error before any request
  is sent.
- `Environment` values other than `sandbox` and `production` are rejected at
  construction.
- Multipart content types, including caller-supplied overrides, are validated
  against a strict media-type pattern so they cannot inject MIME headers.
- A success (2xx) response whose body is not a JSON object now returns
  `UnexpectedResponseError` instead of a raw JSON decoding error.

### Added

- First release of the Go server-side SDK, versioned 12.0.0 to align with the
  V12 mobile SDKs.
- Full V3 API coverage: enhanced KYC, document verification, enhanced document
  verification, biometric KYC, biometric enrollment, authentication and compare,
  verification status with a `WaitUntilComplete` polling helper, callback
  replay, fraud reporting (with `FlagFraud` / `ClearFraud` wrappers), and the
  four services endpoints.
- Internal JWT authentication with a thread-safe cache and a single automatic
  refresh on 401.
- Typed error hierarchy in the root package, covering both API error body
  shapes and usable with `errors.As`.
- Automatic retries with exponential backoff and `Retry-After` support for
  idempotent calls only.
- Consent builder and client-side validation for user details and fraud
  reports.
- Optional HMAC request signing, off unless `PartnerSecret` is configured.
- `UnexpectedResponseError`, returned when a success response body is not a
  JSON object.
- Zero runtime dependencies beyond the Go standard library.

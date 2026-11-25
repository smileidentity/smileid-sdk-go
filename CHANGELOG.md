# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Phone and address verification, matching API spec v3.1215.0:
  - `Phone.Verify` submits a phone number verification (`POST /v3/phone`) with the new `PhoneVerificationParams`. The token carries the `phone_number_verification` product claim.
  - `Address.Verify` submits an address verification (`POST /v3/address`) with the new `AddressVerificationParams`. The token carries the `address_verification` product claim.
  - Both return an `AcceptedResponse` and deliver the result to the callback URL.
- Smile Links, on the new `client.SmileLinks` resource: `Create` (`POST /v3/smile_links`), `List` (`GET /v3/smile_links`), `Get` (`GET /v3/smile_links/{refId}`) and `Update` (`PUT /v3/smile_links/{refId}`). New types: `CreateSmileLinkParams`, `UpdateSmileLinkParams`, `ListSmileLinksParams`, `SmileLinkIDType`, `SmileLinkCreated`, `SmileLink`, `SmileLinkListResponse` and the types they use. The tokens carry no product claim. `Create` and `Update` are not retried.
- `UserID`, `ResponseSignature` and `ResponseTimestamp` on `BasicKYCResult` and `OneTimeAMLResult`, set from the `User-ID`, `Response-Signature` and `Response-Timestamp` headers of a `BasicKYC.VerifySync` or `AML.ScreenSync` response. The SDK does not verify the signature. They are empty on a callback payload.

## [13.2.0] - 2026-11-24

### Added

- Basic KYC operations, matching API spec v3.1215.0, on the new `client.BasicKYC` resource:
  - `BasicKYC.Verify` submits a Basic KYC verification (`POST /v3/basic_kyc`).
  - `BasicKYC.VerifySync` submits one and returns the result inline (`POST /v3/basic_kyc/sync`). It is not retried, because a repeat would bill a second lookup. It sends a callback URL only when the call sets one, never the client's default, because the sync endpoint ignores it.
  - New types: `BasicKYCParams`, `BasicKYCResult`, `BasicKYCMatchResults` and `BasicKYCUserProvidedInfo`.
  - Both operations mint their token with the `basic_kyc` product claim, taken from `x-smile-token-product`.
- `EnhancedKYC.VerifySync`, which submits an Enhanced KYC verification and returns the result inline (`POST /v3/enhanced_kyc/sync`), matching API spec v3.1215.0. It takes the existing `EnhancedKYCParams` and returns the new `EnhancedKYCResult`, the payload the Enhanced KYC callback delivers. Its `JobID` and `UserID` come from the `SmileID-Job-ID` and `User-ID` response headers, falling back to the body's `job_id` and `user_id` when a header is absent or blank. Its `ResponseSignature` and `ResponseTimestamp` hold the `Response-Signature` and `Response-Timestamp` headers, for checking with your own webhook verification code; the SDK does not verify them. Its `Raw` map keeps the whole body, including fields with no typed field. It is not retried, because a repeat would bill a second lookup: after a 408, 424 or 500, read the outcome with `Verifications.Retrieve` using the error's `JobID`. It sends a callback URL only when the call sets one, never the client's default.
- `JobID` on every SDK error (`Error` and the typed errors that embed it), read from the `SmileID-Job-ID` response header without regard to case. It is empty when the response has no such header. Use it after a `BasicKYC.VerifySync` 408, 424 or 500 to read the outcome with `Verifications.Retrieve` instead of resubmitting.
- `JobID` on `BasicKYCResult` and `OneTimeAMLResult`, set from the `SmileID-Job-ID` header of a `BasicKYC.VerifySync` or `AML.ScreenSync` response. It is not part of the body, so it is empty on a callback payload.
- `operations.Request.ResponseHeaders`, which names response headers the transport copies into the decoded result. A blank header is treated as absent.

## [13.1.0] - 2026-11-19

### Added

- AML operations, matching API spec v3.1165.0, on the new `client.AML` resource:
  - `AML.Screen` submits a One-Time AML screening (`POST /v3/aml`).
  - `AML.ScreenSync` submits one and returns the result inline (`POST /v3/aml/sync`). `IdempotencyKey` sets the `SmileID-Idempotency-Key` header.
  - `AML.Monitor` submits an AML Monitoring screening (`POST /v3/aml/monitoring`).
  - `AML.MonitoringNews` reads the adverse-media articles for a person reference (`GET /v3/aml/monitoring/{job_id}/news/{reference}`).

### Changed

- Each operation now carries its token `product` from the spec's `x-smile-token-product` extension (spec v3.1192.0) instead of a hand-kept path table. Token claims are unchanged.

### Fixed

- `AMLPersonMatch` now parses `addresses`, `aliases`, `dates_of_birth` and `nationalities` as lists of strings, as spec v3.1192.0 defines them. They were typed as lists of objects, so a real response failed to parse. The type has not been released.
- AML requests now mint their token with the `product` claim the spec requires: `one_time_aml` for `POST /v3/aml` and `POST /v3/aml/sync`, and `aml` for `POST /v3/aml/monitoring` and the news lookup, as set by `x-smile-token-product` in the spec.

## [13.0.0] - 2026-11-09

This release replaces the unpublished 12.2.0. It changes the public API, so it is a new major version.

### Breaking changes

- The import path is now `github.com/smileidentity/smileid-sdk-go/v13`. Run `go get github.com/smileidentity/smileid-sdk-go/v13` and update your imports.
- `AuthenticationParams.UseEnrolledImage` is removed. Always send `SelfieImage` and `LivenessImages` to `Biometric.Authenticate`; to check a biometric KYC job against the enrolled selfie, pass `usesmileid.WithEnrolledImage()` instead.
- `Biometric.Authenticate` returns a `*ValidationError` without `SelfieImage` and `LivenessImages`. Send both images.
- An empty or whitespace-padded `UserID` returns a `*ValidationError`. Trim the ID before you send it.
- A 401 response is no longer retried with a new token and returns an `*AuthenticationError` straight away. Check your partner ID and API key.
- `operations.Request.UserIDHeader` is now a `*string`. Set it to a pointer if you build requests in the `generated/operations` package yourself.

### Changed

- Each request mints its own token, bound to that request's `user_id` and product, instead of reusing one cached token.

### Added

- `usesmileid.WithEnrolledImage()` verifies a biometric KYC job against the user's enrolled selfie; the grant is bound into the job's token and the selfie and liveness images may be omitted. `UserID` is then required.

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

[Unreleased]: https://github.com/smileidentity/smileid-sdk-go/compare/v13.0.0...HEAD
[13.0.0]: https://github.com/smileidentity/smileid-sdk-go/compare/v12.1.0...v13.0.0
[12.1.0]: https://github.com/smileidentity/smileid-sdk-go/compare/v12.0.0...v12.1.0
[12.0.0]: https://github.com/smileidentity/smileid-sdk-go/releases/tag/v12.0.0

# Smile ID Go SDK

The official Go server-side SDK for the [Smile ID](https://smileidentity.com) V3 APIs. It covers every V3 operation, manages authentication for you, and depends only on the Go standard library.

## Requirements

- Go 1.21 or later.

## Installation

```bash
go get github.com/smileidentity/smileid-sdk-go/v12
```

Import the root package:

```go
import "github.com/smileidentity/smileid-sdk-go/v12"
```

The import path ends in `/v12` because the module is at major version 12. In code you refer to the package as `usesmileid`.

## Authentication

Construct a client with your partner ID and API key. The SDK fetches, caches and refreshes the internal JWT for you; you never handle a token directly.

```go
client, err := usesmileid.NewClient(usesmileid.Config{
    PartnerID: "1234",
    APIKey:    os.Getenv("SMILE_API_KEY"),
})
if err != nil {
    log.Fatal(err)
}
```

Partner ids are displayed zero-padded (for example 002) but must be passed without leading zeros (2).

Keep your API key out of source control. Read it from the environment or a secret manager.

## Environment selection

The client uses the sandbox by default. Set `Environment` to switch to production.

```go
client, err := usesmileid.NewClient(usesmileid.Config{
    PartnerID:   "1234",
    APIKey:      os.Getenv("SMILE_API_KEY"),
    Environment: usesmileid.Production,
})
```

`Environment` accepts only `usesmileid.Sandbox` or `usesmileid.Production` (or the empty zero value, which means sandbox); anything else fails at construction.

Those are the only two named environments. To reach any other host, set `BaseURL`. It wins over `Environment`.

```go
client, err := usesmileid.NewClient(usesmileid.Config{
    PartnerID: "2",
    APIKey:    os.Getenv("SMILE_API_KEY"),
    BaseURL:   "https://your-environment.example.com",
})
```

`Config` also accepts:

- `DefaultCallbackURL` — used when a call omits a callback URL.
- `Timeout` — the per-request timeout (default 30s), enforced through the context.
- `MaxRetries` — retries for idempotent operations (default 2; a negative value disables them).
- `HTTPClient` — an injected `*http.Client` for testing or proxies.

Every method takes a `context.Context` as its first argument and accepts optional per-request options, such as `usesmileid.WithTimeout` and `usesmileid.WithCallbackURL`.

### URLs must be https

`BaseURL` must be an absolute `https` URL with no query or fragment; there is no insecure override. Callback URLs — `DefaultCallbackURL`, the `CallbackURL` params field, and `WithCallbackURL` — must also be absolute `https` URLs. An insecure URL returns a `*ValidationError` before any request is sent.

## Building consent and user details

Every entry endpoint needs consent and user details. Use the `GrantConsent` helper, and set at least one of email or phone number on user details — the SDK checks this before sending.

```go
consent := usesmileid.GrantConsent(time.Now(), "EN", "https://example.com/privacy")

userDetails := usesmileid.UserDetails{
    GivenNames: "Amina Fatou",
    LastName:   "Clearwater",
    Email:      usesmileid.String("amina.clearwater@example.com"),
}
```

Non-production environments match test identities on given names, last name and email. The identity above resolves to `clear`; an unrecognised identity resolves to `block`.

`usesmileid.String`, `usesmileid.Bool`, `usesmileid.Float64` and the generic `usesmileid.Ptr` return pointers for the optional fields on params structs.

## Supplying images

Image fields accept a file path, a byte slice or a reader:

```go
usesmileid.FromFile("selfie.jpg")
usesmileid.FromBytes(buf, "selfie.jpg")
usesmileid.FromReader(r, "selfie.jpg")
```

The SDK sends `image/jpeg` for selfie, liveness and comparison images. For the document and document back it detects PNG by file extension or magic bytes. To force a type, call `WithContentType` on the input.

## Usage

### Enhanced KYC

```go
accepted, err := client.EnhancedKYC.Verify(ctx, usesmileid.EnhancedKYCParams{
    Country:     "NG",
    IDType:      "NIN",
    IDNumber:    "12345678901",
    UserDetails: userDetails,
    Consent:     consent,
})
if err != nil {
    log.Fatal(err)
}
fmt.Println(accepted.JobID, accepted.IsAccepted())
```

### Document verification

```go
accepted, err := client.Documents.Verify(ctx, usesmileid.DocumentVerificationParams{
    Country:        "NG",
    SelfieImage:    usesmileid.FromFile("selfie.jpg"),
    LivenessImages: []*usesmileid.BinaryInput{usesmileid.FromFile("live1.jpg"), usesmileid.FromFile("live2.jpg")},
    Document:       usesmileid.FromFile("document.jpg"),
    UserDetails:    userDetails,
    Consent:        consent,
})
```

### Enhanced document verification

`VerifyEnhanced` requires `IDType`.

```go
accepted, err := client.Documents.VerifyEnhanced(ctx, usesmileid.DocumentVerificationParams{
    Country:        "NG",
    IDType:         usesmileid.String("PASSPORT"),
    SelfieImage:    usesmileid.FromFile("selfie.jpg"),
    LivenessImages: liveness,
    Document:       usesmileid.FromFile("document.jpg"),
    UserDetails:    userDetails,
    Consent:        consent,
})
```

### Biometric KYC

```go
accepted, err := client.BiometricKYC.Verify(ctx, usesmileid.BiometricKYCParams{
    Country:        "NG",
    IDType:         "NIN",
    IDNumber:       "12345678901",
    SelfieImage:    usesmileid.FromFile("selfie.jpg"),
    LivenessImages: liveness,
    UserDetails:    userDetails,
    Consent:        consent,
})
```

### Biometric enrollment

```go
accepted, err := client.Biometric.Enroll(ctx, usesmileid.RegistrationParams{
    SelfieImage:    usesmileid.FromFile("selfie.jpg"),
    LivenessImages: liveness,
    UserDetails:    userDetails,
    Consent:        consent,
})
```

### Biometric authentication

`UserID` is required. Provide images unless `UseEnrolledImage` is true.

```go
accepted, err := client.Biometric.Authenticate(ctx, usesmileid.AuthenticationParams{
    UserID:         "user_123",
    SelfieImage:    usesmileid.FromFile("selfie.jpg"),
    LivenessImages: liveness,
    UserDetails:    userDetails,
    Consent:        consent,
})
```

### Biometric compare

```go
accepted, err := client.Biometric.Compare(ctx, usesmileid.CompareParams{
    SelfieImage:         usesmileid.FromFile("selfie.jpg"),
    ComparisonImage:     usesmileid.FromFile("id_photo.jpg"),
    ComparisonImageType: usesmileid.ComparisonImageTypeIDPhoto,
    UserDetails:         userDetails,
    Consent:             consent,
})
```

### Retrieve a verification

`Retrieve` returns a `JobStatus`. A job that is not found comes back with status `not_found` rather than as an error, so polling can tell the two apart.

```go
status, err := client.Verifications.Retrieve(ctx, "job_...")
if err != nil {
    log.Fatal(err)
}
fmt.Println(status.Status, status.Message)
// clear Job completed
```

`Status` is `processing` while the job runs, `not_found` for an unknown job, and otherwise the decision itself: `clear`, `block`, `attention` or `error`. `Message` is human-readable text, not the decision. `status.IsComplete()` reports whether the job reached a decision.

### Wait for completion

```go
status, err := client.Verifications.WaitUntilComplete(ctx, "job_...", usesmileid.WaitOptions{
    Interval: 2 * time.Second,
    Timeout:  60 * time.Second,
})
```

`WaitUntilComplete` polls while the status is `processing` and returns as soon as the job reaches a decision, whichever one it is.

`WaitOptions` defaults to a 2s interval, a 60s timeout, and treating a not-found job as still pending. Set `TreatNotFoundAsPending: usesmileid.Bool(false)` to return a not-found status straight away. On timeout the method returns a `*usesmileid.TimeoutError`.

### Replay a callback

```go
replay, err := client.Verifications.Replay(ctx, "job_...", usesmileid.ReplayParams{
    CallbackURL: usesmileid.String("https://partner.example.com/webhook"),
})
```

### Report, flag and clear fraud

```go
_, err := client.Users.ReportFraud(ctx, "user_123", usesmileid.ReportFraudParams{
    IsFraud:    true,
    Reason:     usesmileid.String(usesmileid.ReasonAccountTakeover),
    ReportedBy: "risk@partner.example",
})

_, err = client.Users.FlagFraud(ctx, "user_123", usesmileid.FlagFraudParams{
    Reason:     usesmileid.ReasonFirstPartyFraud,
    ReportedBy: "risk@partner.example",
})

_, err = client.Users.ClearFraud(ctx, "user_123", usesmileid.ClearFraudParams{
    Notes:      "Cleared by appeals review",
    ReportedBy: "risk@partner.example",
})
```

A fraud report needs a reason when `IsFraud` is true, and notes when it is false or the reason is `OTHER`. The SDK enforces these rules before sending.

### Services

The bank codes, supported ID types and supported documents endpoints need no authentication. The ID status endpoint does.

```go
banks, err := client.Services.BankCodes(ctx, usesmileid.BankCodesParams{
    Country: usesmileid.String("NG"),
})

idTypes, err := client.Services.SupportedIDTypes(ctx, usesmileid.SupportedIDTypesParams{
    Country: usesmileid.String("NG"),
})

docs, err := client.Services.SupportedDocuments(ctx, usesmileid.SupportedDocumentsParams{
    CountryCode: usesmileid.String("NG"),
})

idStatus, err := client.Services.IDStatus(ctx, usesmileid.IDStatusParams{
    Country: "NG",
    IDType:  "NIN",
})
```

## Error handling

Every failure returns a typed error over a shared base. Match a specific type with `errors.As` and read the fields you need.

```go
accepted, err := client.EnhancedKYC.Verify(ctx, params)
if err != nil {
    var invalid *usesmileid.InvalidRequestError
    var auth *usesmileid.AuthenticationError
    var rateLimit *usesmileid.RateLimitError
    switch {
    case errors.As(err, &invalid):
        log.Printf("bad request: %s (HTTP %d)", invalid.Message, invalid.StatusCode)
    case errors.As(err, &auth):
        log.Printf("authentication failed: %s", auth.Message)
    case errors.As(err, &rateLimit):
        log.Printf("rate limited: %s", rateLimit.Message)
    default:
        log.Printf("request failed: %v", err)
    }
}
```

The error types are:

| Type | Raised for |
|------|------------|
| `InvalidRequestError` | HTTP 400 and 415 |
| `AuthenticationError` | HTTP 401 |
| `PaymentRequiredError` | HTTP 402 |
| `PermissionError` | HTTP 403, including the services `{error, code}` shape |
| `NotFoundError` | HTTP 404 (never raised by `Retrieve`) |
| `ConflictError` | HTTP 409 (for example a replay of a job still processing) |
| `PayloadTooLargeError` | HTTP 413 |
| `RateLimitError` | HTTP 429 |
| `APIError` | HTTP 5xx |
| `Error` (base) | any other unmapped status |
| `UnexpectedResponseError` | a success (2xx) response whose body is not a JSON object |
| `ConnectionError` | network failure, timeout or context cancellation |
| `ValidationError` | client-side validation, before any request is sent |
| `TimeoutError` | `WaitUntilComplete` exceeding its timeout |

Every error exposes `StatusCode`, `Status`, `Message`, `Code`, `RequestID` and `RawBody`. `Code` is populated only on the services `{error, code}` responses.

## Telemetry

The SDK sends `SmileID-Source-SDK: go`, `SmileID-Source-SDK-Version` and a `User-Agent` header on every request. These are observability signals, never authentication. There is no way to disable them.

## Licence

[MIT](LICENSE).

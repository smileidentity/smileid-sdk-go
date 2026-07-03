// Package smileid is the Go server-side SDK for the Smile ID V3 APIs.
//
// Construct a client with [NewClient] and a [Config], then call operations
// through the resource namespaces on the client:
//
//	client, err := smileid.NewClient(smileid.Config{
//		PartnerID: "1234",
//		APIKey:    os.Getenv("SMILE_API_KEY"),
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	accepted, err := client.EnhancedKYC.Verify(ctx, smileid.EnhancedKYCParams{
//		Country:  "NG",
//		IDType:   "NIN",
//		IDNumber: "12345678901",
//		UserDetails: smileid.UserDetails{
//			GivenNames: "Amina Fatou",
//			LastName:   "Clearwater",
//			Email:      smileid.String("amina.clearwater@example.com"),
//		},
//		Consent: smileid.GrantConsent(time.Now(), "EN", "https://example.com/privacy"),
//	})
//
// Authentication is internal: the SDK fetches and caches a JWT and never
// exposes it. The client defaults to the sandbox environment. Errors are a
// typed hierarchy over [Error]; match a specific type with errors.As.
package smileid

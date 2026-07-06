package usesmileid

import "net/http"

// Client is the Smile ID V3 API client. Construct one with [NewClient] and
// call operations through the resource namespaces (EnhancedKYC, Documents,
// BiometricKYC, Biometric, Verifications, Users, Services).
type Client struct {
	cfg       config
	transport *transport

	// EnhancedKYC covers POST /v3/enhanced_kyc.
	EnhancedKYC *EnhancedKYCResource
	// Documents covers document verification and enhanced document verification.
	Documents *DocumentsResource
	// BiometricKYC covers POST /v3/biometric_kyc.
	BiometricKYC *BiometricKYCResource
	// Biometric covers enrollment, authentication and compare.
	Biometric *BiometricResource
	// Verifications covers status retrieval, the polling helper, and replay.
	Verifications *VerificationsResource
	// Users covers fraud reporting and its convenience wrappers.
	Users *UsersResource
	// Services covers the four services endpoints.
	Services *ServicesResource
}

// NewClient validates the configuration and returns a ready client. It returns
// a *ValidationError if PartnerID or APIKey is missing or malformed.
func NewClient(cfg Config) (*Client, error) {
	normalized, err := cfg.normalize()
	if err != nil {
		return nil, err
	}

	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{}
	}

	c := &Client{cfg: normalized, transport: newTransport(normalized, hc)}
	c.EnhancedKYC = &EnhancedKYCResource{c}
	c.Documents = &DocumentsResource{c}
	c.BiometricKYC = &BiometricKYCResource{c}
	c.Biometric = &BiometricResource{c}
	c.Verifications = &VerificationsResource{c}
	c.Users = &UsersResource{c}
	c.Services = &ServicesResource{c}
	return c, nil
}

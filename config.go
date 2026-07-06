package usesmileid

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Environment selects the Smile ID base URL.
type Environment string

const (
	// Sandbox is the testing environment (the default).
	Sandbox Environment = "sandbox"
	// Production is the live environment.
	Production Environment = "production"
)

const (
	sandboxBaseURL    = "https://testapi.smileidentity.com"
	productionBaseURL = "https://api.smileidentity.com"

	defaultTimeout    = 30 * time.Second
	defaultMaxRetries = 2
)

var partnerIDPattern = regexp.MustCompile(`^[1-9]\d*$`)

// Config configures a [Client]. PartnerID and APIKey are required; everything
// else has a sensible default.
type Config struct {
	// PartnerID is the numeric partner identifier (no leading zeros).
	PartnerID string
	// APIKey is the partner API key.
	APIKey string
	// Environment selects the base URL. Defaults to Sandbox.
	Environment Environment
	// DefaultCallbackURL is used when a call omits a callback URL. It must be
	// an absolute https URL.
	DefaultCallbackURL string
	// BaseURL overrides the environment-derived base URL when set. It must be
	// an absolute https URL with no query or fragment.
	BaseURL string
	// Timeout is the per-request total timeout. Defaults to 30s.
	Timeout time.Duration
	// MaxRetries is the number of retries for idempotent operations.
	// Defaults to 2. A negative value disables retries.
	MaxRetries int
	// HTTPClient is the HTTP client used for requests. Defaults to a fresh
	// *http.Client. Per-request timeouts are enforced via context, so an
	// injected client's own Timeout is left untouched.
	HTTPClient *http.Client
}

// config is the normalized, internal view of Config.
type config struct {
	partnerID          string
	apiKey             string
	defaultCallbackURL string
	baseURL            string
	timeout            time.Duration
	maxRetries         int
}

func (c Config) normalize() (config, error) {
	if c.PartnerID == "" {
		return config{}, validationErrorf("partner_id is required")
	}
	if !partnerIDPattern.MatchString(c.PartnerID) {
		return config{}, validationErrorf("partner_id must be a numeric string with no leading zeros")
	}
	if c.APIKey == "" {
		return config{}, validationErrorf("api_key is required")
	}

	env := c.Environment
	switch env {
	case "":
		env = Sandbox
	case Sandbox, Production:
	default:
		return config{}, validationErrorf("environment must be %q or %q", Sandbox, Production)
	}

	base := c.BaseURL
	if base == "" {
		if env == Production {
			base = productionBaseURL
		} else {
			base = sandboxBaseURL
		}
	}
	if err := validateBaseURL(base); err != nil {
		return config{}, err
	}
	base = strings.TrimRight(base, "/")

	if c.DefaultCallbackURL != "" {
		if err := validateHTTPSURL("default_callback_url", c.DefaultCallbackURL); err != nil {
			return config{}, err
		}
	}

	timeout := c.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	retries := c.MaxRetries
	switch {
	case retries == 0:
		retries = defaultMaxRetries
	case retries < 0:
		retries = 0
	}

	return config{
		partnerID:          c.PartnerID,
		apiKey:             c.APIKey,
		defaultCallbackURL: c.DefaultCallbackURL,
		baseURL:            base,
		timeout:            timeout,
		maxRetries:         retries,
	}, nil
}

// validateBaseURL requires an absolute https URL with no query or fragment.
// There is deliberately no insecure escape hatch.
func validateBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return validationErrorf("base_url must be an absolute https URL")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return validationErrorf("base_url must not contain a query or fragment")
	}
	return nil
}

// validateHTTPSURL requires an absolute https URL. Used for callback URLs.
func validateHTTPSURL(name, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return validationErrorf("%s must be an absolute https URL", name)
	}
	return nil
}

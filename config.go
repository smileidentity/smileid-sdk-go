package smileid

import (
	"net"
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
	// PartnerSecret enables HMAC request signing when set.
	PartnerSecret string
	// DefaultCallbackURL is used when a call omits a callback URL.
	DefaultCallbackURL string
	// BaseURL overrides the environment-derived base URL when set.
	BaseURL string
	// AllowInsecureBaseURL permits http:// loopback BaseURL values for local
	// tests. Production code should leave this false.
	AllowInsecureBaseURL bool
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
	partnerSecret      string
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
	base, err := normalizeBaseURL(base, c.AllowInsecureBaseURL)
	if err != nil {
		return config{}, err
	}

	if c.DefaultCallbackURL != "" {
		if err := validateCallbackURL(c.DefaultCallbackURL); err != nil {
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
		partnerSecret:      c.PartnerSecret,
		defaultCallbackURL: c.DefaultCallbackURL,
		baseURL:            base,
		timeout:            timeout,
		maxRetries:         retries,
	}, nil
}

func normalizeBaseURL(raw string, allowInsecure bool) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", validationErrorf("base_url must be an absolute URL")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", validationErrorf("base_url must not include query or fragment")
	}
	if u.Scheme == "https" {
		return strings.TrimRight(u.String(), "/"), nil
	}
	if allowInsecure && u.Scheme == "http" && isLoopbackHost(u.Hostname()) {
		return strings.TrimRight(u.String(), "/"), nil
	}
	return "", validationErrorf("base_url must use https")
}

func validateCallbackURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return validationErrorf("callback_url must be an absolute URL")
	}
	if u.Scheme != "https" {
		return validationErrorf("callback_url must use https")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

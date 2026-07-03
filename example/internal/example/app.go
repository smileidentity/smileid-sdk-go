package example

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	smileid "github.com/smileidentity/smileid-sdk-go/v12"
)

const defaultPrivacyURL = "https://example.com/privacy"

type servicesOutput struct {
	Country   string                  `json:"country"`
	BankCodes []smileid.BankCode      `json:"bank_codes"`
	IDTypes   []smileid.IDType        `json:"id_types"`
	Documents []smileid.ValidDocument `json:"documents"`
}

type acceptedOutput struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	JobID    string `json:"job_id"`
	UserID   string `json:"user_id"`
	Accepted bool   `json:"accepted"`
}

type statusOutput struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	JobID   string `json:"job_id"`
	UserID  string `json:"user_id"`
}

type replayOutput struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	JobID   string `json:"job_id"`
	UserID  string `json:"user_id"`
}

type appConfig struct {
	partnerID          string
	apiKey             string
	partnerSecret      string
	baseURL            string
	callbackURL        string
	timeout            time.Duration
	insecureTLSForTest bool
}

// Run executes the example CLI. It is factored for tests so the same code path
// drives both the command and the integration-style fake server checks.
func Run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) error {
	if getenv == nil {
		getenv = os.Getenv
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	cfg, rest, err := parseGlobalFlags(args, getenv, stderr)
	if err != nil {
		return err
	}
	if len(rest) == 0 {
		return usageError("missing command; run one of: services, enhanced-kyc, status, replay")
	}
	switch rest[0] {
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	}
	if err := validateConfig(cfg); err != nil {
		return err
	}

	client, err := newClient(cfg)
	if err != nil {
		return err
	}

	switch rest[0] {
	case "services":
		return runServices(ctx, client, rest[1:], stdout, stderr)
	case "enhanced-kyc":
		return runEnhancedKYC(ctx, client, cfg, rest[1:], stdout, stderr)
	case "status":
		return runStatus(ctx, client, rest[1:], stdout, stderr)
	case "replay":
		return runReplay(ctx, client, rest[1:], stdout, stderr)
	default:
		return usageError("unknown command %q", rest[0])
	}
}

func parseGlobalFlags(args []string, getenv func(string) string, stderr io.Writer) (appConfig, []string, error) {
	cfg := appConfig{
		partnerID:     getenv("SMILE_PARTNER_ID"),
		apiKey:        getenv("SMILE_API_KEY"),
		partnerSecret: getenv("SMILE_PARTNER_SECRET"),
		baseURL:       getenv("SMILE_BASE_URL"),
		callbackURL:   getenv("SMILE_CALLBACK_URL"),
		timeout:       30 * time.Second,
	}
	if raw := getenv("SMILE_TIMEOUT"); raw != "" {
		timeout, err := time.ParseDuration(raw)
		if err != nil {
			return appConfig{}, nil, fmt.Errorf("SMILE_TIMEOUT must be a Go duration such as 30s: %w", err)
		}
		cfg.timeout = timeout
	}
	cfg.insecureTLSForTest = getenv("SMILE_EXAMPLE_INSECURE_TLS") == "1"

	flags := flag.NewFlagSet("smileid-example-go", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&cfg.partnerID, "partner-id", cfg.partnerID, "Smile ID partner ID")
	flags.StringVar(&cfg.apiKey, "api-key", cfg.apiKey, "Smile ID API key")
	flags.StringVar(&cfg.partnerSecret, "partner-secret", cfg.partnerSecret, "optional Smile ID HMAC partner secret")
	flags.StringVar(&cfg.baseURL, "base-url", cfg.baseURL, "optional Smile ID API base URL override")
	flags.StringVar(&cfg.callbackURL, "callback-url", cfg.callbackURL, "default callback URL for verification commands")
	flags.DurationVar(&cfg.timeout, "timeout", cfg.timeout, "per-request timeout")
	if err := flags.Parse(args); err != nil {
		return appConfig{}, nil, err
	}
	return cfg, flags.Args(), nil
}

func validateConfig(cfg appConfig) error {
	var missing []string
	if strings.TrimSpace(cfg.partnerID) == "" {
		missing = append(missing, "SMILE_PARTNER_ID or --partner-id")
	}
	if strings.TrimSpace(cfg.apiKey) == "" {
		missing = append(missing, "SMILE_API_KEY or --api-key")
	}
	if len(missing) > 0 {
		return usageError("missing %s", strings.Join(missing, " and "))
	}
	return nil
}

func newClient(cfg appConfig) (*smileid.Client, error) {
	sdkConfig := smileid.Config{
		PartnerID:          cfg.partnerID,
		APIKey:             cfg.apiKey,
		PartnerSecret:      cfg.partnerSecret,
		BaseURL:            cfg.baseURL,
		DefaultCallbackURL: cfg.callbackURL,
		Timeout:            cfg.timeout,
	}
	if cfg.insecureTLSForTest {
		sdkConfig.HTTPClient = &http.Client{Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}}
	}
	return smileid.NewClient(sdkConfig)
}

func runServices(ctx context.Context, client *smileid.Client, args []string, stdout, stderr io.Writer) error {
	country := "NG"
	flags := flag.NewFlagSet("services", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&country, "country", country, "ISO 3166-1 alpha-2 country code")
	if err := flags.Parse(args); err != nil {
		return err
	}

	banks, err := client.Services.BankCodes(ctx, smileid.BankCodesParams{Country: smileid.String(country)})
	if err != nil {
		return fmt.Errorf("list bank codes: %w", err)
	}
	idTypes, err := client.Services.SupportedIDTypes(ctx, smileid.SupportedIDTypesParams{Country: smileid.String(country)})
	if err != nil {
		return fmt.Errorf("list supported ID types: %w", err)
	}
	docs, err := client.Services.SupportedDocuments(ctx, smileid.SupportedDocumentsParams{CountryCode: smileid.String(country)})
	if err != nil {
		return fmt.Errorf("list supported documents: %w", err)
	}
	return encodeJSON(stdout, servicesOutput{
		Country:   country,
		BankCodes: banks.BankCodes,
		IDTypes:   idTypes.IDTypes,
		Documents: docs.ValidDocuments,
	})
}

func runEnhancedKYC(ctx context.Context, client *smileid.Client, cfg appConfig, args []string, stdout, stderr io.Writer) error {
	params := struct {
		country    string
		idType     string
		idNumber   string
		givenNames string
		lastName   string
		email      string
		phone      string
		privacyURL string
		language   string
		callback   string
		userID     string
	}{
		country:    "NG",
		privacyURL: defaultPrivacyURL,
		language:   "EN",
		callback:   cfg.callbackURL,
	}
	flags := flag.NewFlagSet("enhanced-kyc", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&params.country, "country", params.country, "ISO 3166-1 alpha-2 country code")
	flags.StringVar(&params.idType, "id-type", params.idType, "Smile ID ID type, for example NIN")
	flags.StringVar(&params.idNumber, "id-number", params.idNumber, "subject ID number")
	flags.StringVar(&params.givenNames, "given-names", params.givenNames, "subject given names")
	flags.StringVar(&params.lastName, "last-name", params.lastName, "subject last name")
	flags.StringVar(&params.email, "email", params.email, "subject email address")
	flags.StringVar(&params.phone, "phone-number", params.phone, "subject phone number")
	flags.StringVar(&params.privacyURL, "privacy-url", params.privacyURL, "privacy notice URL shown to the subject")
	flags.StringVar(&params.language, "notice-language", params.language, "privacy notice language")
	flags.StringVar(&params.callback, "callback-url", params.callback, "callback URL for this verification")
	flags.StringVar(&params.userID, "user-id", params.userID, "optional partner user ID")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if params.idType == "" || params.idNumber == "" {
		return usageError("enhanced-kyc requires --id-type and --id-number")
	}

	userDetails := smileid.UserDetails{
		GivenNames: params.givenNames,
		LastName:   params.lastName,
	}
	if params.email != "" {
		userDetails.Email = smileid.String(params.email)
	}
	if params.phone != "" {
		userDetails.PhoneNumber = smileid.String(params.phone)
	}
	request := smileid.EnhancedKYCParams{
		Country:     params.country,
		IDType:      params.idType,
		IDNumber:    params.idNumber,
		UserDetails: userDetails,
		Consent:     smileid.GrantConsent(time.Now(), params.language, params.privacyURL),
	}
	if params.callback != "" {
		request.CallbackURL = smileid.String(params.callback)
	}
	if params.userID != "" {
		request.UserID = smileid.String(params.userID)
	}

	accepted, err := client.EnhancedKYC.Verify(ctx, request)
	if err != nil {
		return fmt.Errorf("submit enhanced KYC verification: %w", err)
	}
	return encodeJSON(stdout, acceptedOutput{
		Status:   accepted.Status,
		Message:  accepted.Message,
		JobID:    accepted.JobID,
		UserID:   accepted.UserID,
		Accepted: accepted.IsAccepted(),
	})
}

func runStatus(ctx context.Context, client *smileid.Client, args []string, stdout, stderr io.Writer) error {
	jobID := ""
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&jobID, "job-id", jobID, "Smile ID job ID")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if jobID == "" && flags.NArg() > 0 {
		jobID = flags.Arg(0)
	}
	if jobID == "" {
		return usageError("status requires --job-id")
	}
	status, err := client.Verifications.Retrieve(ctx, jobID)
	if err != nil {
		return fmt.Errorf("retrieve verification status: %w", err)
	}
	return encodeJSON(stdout, statusOutput{
		Status: status.Status, Message: status.Message, JobID: status.JobID, UserID: status.UserID,
	})
}

func runReplay(ctx context.Context, client *smileid.Client, args []string, stdout, stderr io.Writer) error {
	jobID := ""
	callbackURL := ""
	flags := flag.NewFlagSet("replay", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&jobID, "job-id", jobID, "Smile ID job ID")
	flags.StringVar(&callbackURL, "callback-url", callbackURL, "callback URL to replay to")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if jobID == "" && flags.NArg() > 0 {
		jobID = flags.Arg(0)
	}
	if jobID == "" {
		return usageError("replay requires --job-id")
	}
	params := smileid.ReplayParams{}
	if callbackURL != "" {
		params.CallbackURL = smileid.String(callbackURL)
	}
	replay, err := client.Verifications.Replay(ctx, jobID, params)
	if err != nil {
		return fmt.Errorf("replay callback: %w", err)
	}
	return encodeJSON(stdout, replayOutput{
		Status: replay.Status, Message: replay.Message, JobID: replay.JobID, UserID: replay.UserID,
	})
}

func encodeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage:
  smileid-example-go [global flags] services --country NG
  smileid-example-go [global flags] enhanced-kyc --country NG --id-type NIN --id-number 12345678901 --given-names Amina --last-name Okafor --email amina@example.com --privacy-url https://example.com/privacy
  smileid-example-go [global flags] status --job-id job_...
  smileid-example-go [global flags] replay --job-id job_... --callback-url https://example.com/webhook

Global flags can also be set with SMILE_PARTNER_ID, SMILE_API_KEY, SMILE_PARTNER_SECRET, SMILE_BASE_URL, SMILE_CALLBACK_URL and SMILE_TIMEOUT.`)
}

type cliUsageError string

func (e cliUsageError) Error() string { return string(e) }

func newUsageError(format string, args ...any) cliUsageError {
	return cliUsageError(fmt.Sprintf(format, args...))
}

func usageErrorf(format string, args ...any) error {
	return newUsageError(format, args...)
}

func usageError(format string, args ...any) error {
	return usageErrorf(format, args...)
}

func IsUsageError(err error) bool {
	var target cliUsageError
	return errors.As(err, &target)
}

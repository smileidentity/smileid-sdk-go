package operations

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/smileidentity/smileid-sdk-go/v13/generated/models"
)

// These tests pin the exact Request each operation builds: method, path,
// transport flags, query, User-ID header and the ordered multipart parts.

type recorder struct {
	req *Request
	out interface{}
	err error
}

func (r *recorder) Do(_ context.Context, req *Request, out interface{}) error {
	r.req = req
	r.out = out
	return r.err
}

// shape is the comparable summary of a Request.
type shape struct {
	Method               string
	Path                 string
	Query                string
	UserIDHeader         string
	Authenticated        bool
	Idempotent           bool
	NeedsPartnerIDHeader bool
	NotFoundReturnsBody  bool
	BodyKind             BodyKind
	Parts                []string
}

func describe(t *testing.T, r *Request) shape {
	t.Helper()
	s := shape{
		Method:               r.Method,
		Path:                 r.Path,
		Query:                r.Query.Encode(),
		Authenticated:        r.Authenticated,
		Idempotent:           r.Idempotent,
		NeedsPartnerIDHeader: r.NeedsPartnerIDHeader,
		NotFoundReturnsBody:  r.NotFoundReturnsBody,
		BodyKind:             r.BodyKind,
	}
	if r.UserIDHeader != nil {
		s.UserIDHeader = *r.UserIDHeader
	}
	if len(r.Headers) != 0 {
		t.Errorf("unexpected operation headers: %v", r.Headers)
	}
	if len(r.ResponseHeaders) != 0 {
		t.Errorf("unexpected response headers: %v", r.ResponseHeaders)
	}
	for _, p := range r.Parts {
		switch p.Kind {
		case PartScalar:
			s.Parts = append(s.Parts, p.Name+"="+p.Scalar)
		case PartJSON:
			b, err := json.Marshal(p.JSON)
			if err != nil {
				t.Fatalf("marshal %s: %v", p.Name, err)
			}
			s.Parts = append(s.Parts, p.Name+" json="+string(b))
		case PartBinary:
			s.Parts = append(s.Parts, p.Name+" file="+p.Binary.Filename())
		case PartBinaryArray:
			names := make([]string, 0, len(p.Array))
			for _, b := range p.Array {
				names = append(names, b.Filename())
			}
			s.Parts = append(s.Parts, p.Name+" files="+strings.Join(names, ","))
		default:
			t.Fatalf("unknown part kind %d", p.Kind)
		}
	}
	return s
}

func check(t *testing.T, rec *recorder, err error, wantOut interface{}, want shape) {
	t.Helper()
	if err != nil {
		t.Fatalf("operation returned %v", err)
	}
	if rec.req == nil {
		t.Fatal("operation did not call Do")
	}
	if rec.out != wantOut {
		t.Errorf("out passed to Do = %p, want %p", rec.out, wantOut)
	}
	if got := describe(t, rec.req); !reflect.DeepEqual(got, want) {
		t.Errorf("request mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

// takeResponseHeaders checks that an operation asks for exactly the want
// response headers, each copied into the given result field, and clears
// ResponseHeaders so describe can check the rest of the request.
func takeResponseHeaders(t *testing.T, r *Request, want map[string]*string) {
	t.Helper()
	if r == nil {
		t.Fatal("operation did not call Do")
	}
	if len(r.ResponseHeaders) != len(want) {
		t.Fatalf("response headers = %v, want %d headers", r.ResponseHeaders, len(want))
	}
	for name, field := range want {
		if r.ResponseHeaders[name] != field {
			t.Errorf("response header %s is not copied into its result field", name)
		}
	}
	r.ResponseHeaders = nil
}

func str(s string) *string                { return &s }
func flt(f float64) *float64              { return &f }
func boolean(b bool) *bool                { return &b }
func img(name string) *models.BinaryInput { return models.FromBytes([]byte(name), name) }

var (
	ctx         = context.Background()
	userDetails = models.UserDetails{GivenNames: "Ada", LastName: "Lovelace", Email: str("ada@example.com")}
	consent     = models.Consent{Granted: true, GrantedAt: "2026-01-02T03:04:05Z", NoticeLanguage: "en", NoticePrivacyPolicyURL: "https://example.com/p"}
	partner     = map[string]string{"job_id": "j1"}
	metadata    = []models.MetadataEntry{{Name: "k", Value: "v"}}
	liveness    = []*models.BinaryInput{img("l1.jpg"), img("l2.jpg")}
)

const (
	userDetailsJSON = `user_details json={"given_names":"Ada","last_name":"Lovelace","email":"ada@example.com"}`
	consentJSON     = `consent json={"granted":true,"granted_at":"2026-01-02T03:04:05Z","notice_language":"en","notice_privacy_policy_url":"https://example.com/p"}`
	partnerJSON     = `partner_params json={"job_id":"j1"}`
	metadataJSON    = `metadata json=[{"name":"k","value":"v"}]`
)

func TestDoErrorIsReturned(t *testing.T) {
	want := errors.New("boom")
	rec := &recorder{err: want}
	if err := RetrieveStatus(ctx, rec, "job", &models.JobStatus{}); !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

func TestEnhancedKYC(t *testing.T) {
	base := shape{Method: "POST", Path: "/v3/enhanced_kyc", Authenticated: true, BodyKind: BodyMultipart}

	t.Run("all fields", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := EnhancedKYC(ctx, rec, models.EnhancedKYCParams{
			Country: "NG", IDType: "BVN", IDNumber: "123", UserDetails: userDetails, Consent: consent,
			CallbackURL: str("https://cb"), BankCode: str("058"), Operator: str("MTN"),
			PartnerParams: partner, Metadata: metadata, UserID: str("u1"),
		}, out)
		want := base
		want.UserIDHeader = "u1"
		want.Parts = []string{"country=NG", "id_type=BVN", "id_number=123", userDetailsJSON, consentJSON,
			"callback_url=https://cb", "bank_code=058", "operator=MTN", partnerJSON, metadataJSON}
		check(t, rec, err, out, want)
	})

	t.Run("required fields only", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := EnhancedKYC(ctx, rec, models.EnhancedKYCParams{
			Country: "NG", IDType: "NIN", IDNumber: "1", UserDetails: userDetails, Consent: consent,
			Metadata: []models.MetadataEntry{},
		}, out)
		want := base
		want.Parts = []string{"country=NG", "id_type=NIN", "id_number=1", userDetailsJSON, consentJSON}
		check(t, rec, err, out, want)
	})

	t.Run("sync", func(t *testing.T) {
		rec, out := &recorder{}, &models.EnhancedKYCResult{}
		err := EnhancedKYCSync(ctx, rec, models.EnhancedKYCParams{
			Country: "NG", IDType: "BVN", IDNumber: "123", UserDetails: userDetails, Consent: consent,
			CallbackURL: str("https://cb"), BankCode: str("058"), Operator: str("MTN"),
			PartnerParams: partner, Metadata: metadata, UserID: str("u1"),
		}, out)
		if rec.req == nil {
			t.Fatal("operation did not call Do")
		}
		headers := rec.req.ResponseHeaders
		wantHeaders := map[string]*string{
			"SmileID-Job-ID":     &out.JobID,
			"User-ID":            &out.UserID,
			"Response-Signature": &out.ResponseSignature,
			"Response-Timestamp": &out.ResponseTimestamp,
		}
		if len(headers) != len(wantHeaders) {
			t.Fatalf("response headers = %v", headers)
		}
		for name, field := range wantHeaders {
			if headers[name] != field {
				t.Errorf("response header %s is not copied into its result field", name)
			}
		}
		rec.req.ResponseHeaders = nil
		want := shape{Method: "POST", Path: "/v3/enhanced_kyc/sync", Authenticated: true, BodyKind: BodyMultipart, UserIDHeader: "u1"}
		want.Parts = []string{"country=NG", "id_type=BVN", "id_number=123", userDetailsJSON, consentJSON,
			"callback_url=https://cb", "bank_code=058", "operator=MTN", partnerJSON, metadataJSON}
		check(t, rec, err, out, want)
		if rec.req.Product != "enhanced_kyc" || rec.req.Idempotent {
			t.Errorf("product = %q, idempotent = %v", rec.req.Product, rec.req.Idempotent)
		}
	})
}

func TestBasicKYC(t *testing.T) {
	p := models.BasicKYCParams{
		Country: "NG", IDType: "NIN", IDNumber: "1", UserDetails: userDetails, Consent: consent,
		CallbackURL: str("https://cb"), BankCode: str("058"), Operator: str("MTN"),
		PartnerParams: partner, Metadata: metadata,
	}
	parts := []string{"country=NG", "id_type=NIN", "id_number=1", userDetailsJSON, consentJSON,
		"callback_url=https://cb", "bank_code=058", "operator=MTN", partnerJSON, metadataJSON}

	t.Run("async", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := BasicKYC(ctx, rec, p, out)
		check(t, rec, err, out, shape{Method: "POST", Path: "/v3/basic_kyc", Authenticated: true, BodyKind: BodyMultipart, Parts: parts})
		if rec.req.Product != "basic_kyc" {
			t.Errorf("product = %q", rec.req.Product)
		}
	})

	t.Run("sync", func(t *testing.T) {
		rec, out := &recorder{}, &models.BasicKYCResult{}
		err := BasicKYCSync(ctx, rec, p, out)
		takeResponseHeaders(t, rec.req, map[string]*string{
			"SmileID-Job-ID": &out.JobID, "User-ID": &out.UserID,
			"Response-Signature": &out.ResponseSignature, "Response-Timestamp": &out.ResponseTimestamp,
		})
		check(t, rec, err, out, shape{Method: "POST", Path: "/v3/basic_kyc/sync", Authenticated: true, BodyKind: BodyMultipart, Parts: parts})
		if rec.req.Product != "basic_kyc" || rec.req.Idempotent {
			t.Errorf("product = %q, idempotent = %v", rec.req.Product, rec.req.Idempotent)
		}
	})

	t.Run("required fields only", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := BasicKYC(ctx, rec, models.BasicKYCParams{
			Country: "NG", IDType: "NIN", IDNumber: "1", UserDetails: userDetails, Consent: consent,
		}, out)
		want := shape{Method: "POST", Path: "/v3/basic_kyc", Authenticated: true, BodyKind: BodyMultipart,
			Parts: []string{"country=NG", "id_type=NIN", "id_number=1", userDetailsJSON, consentJSON}}
		check(t, rec, err, out, want)
	})
}

func TestDocumentVerification(t *testing.T) {
	full := models.DocumentVerificationParams{
		SelfieImage: img("selfie.jpg"), LivenessImages: liveness, Document: img("front.png"),
		DocumentBack: img("back.png"), Consent: consent, Country: "KE", IDType: str("PASSPORT"),
		UserDetails: userDetails, CallbackURL: str("https://cb"), PartnerParams: partner,
		Metadata: metadata, UserID: str("u2"),
	}
	fullParts := []string{"country=KE", "id_type=PASSPORT", "selfie_image file=selfie.jpg",
		"liveness_images files=l1.jpg,l2.jpg", "document file=front.png", "document_back file=back.png",
		userDetailsJSON, consentJSON, "callback_url=https://cb", partnerJSON, metadataJSON}
	minimal := models.DocumentVerificationParams{
		SelfieImage: img("selfie.jpg"), LivenessImages: liveness, Document: img("front.png"),
		Consent: consent, Country: "KE", UserDetails: userDetails,
	}
	minimalParts := []string{"country=KE", "selfie_image file=selfie.jpg", "liveness_images files=l1.jpg,l2.jpg",
		"document file=front.png", userDetailsJSON, consentJSON}

	ops := map[string]func(context.Context, Doer, models.DocumentVerificationParams, *models.AcceptedResponse) error{
		"/v3/document_verification":          DocumentVerification,
		"/v3/enhanced_document_verification": EnhancedDocumentVerification,
	}
	for path, op := range ops {
		base := shape{Method: "POST", Path: path, Authenticated: true, NeedsPartnerIDHeader: true, BodyKind: BodyMultipart}
		t.Run(path+" all fields", func(t *testing.T) {
			rec, out := &recorder{}, &models.AcceptedResponse{}
			err := op(ctx, rec, full, out)
			want := base
			want.UserIDHeader = "u2"
			want.Parts = fullParts
			check(t, rec, err, out, want)
		})
		t.Run(path+" required fields only", func(t *testing.T) {
			rec, out := &recorder{}, &models.AcceptedResponse{}
			err := op(ctx, rec, minimal, out)
			want := base
			want.Parts = minimalParts
			check(t, rec, err, out, want)
		})
	}
}

func TestResidencyDocumentVerification(t *testing.T) {
	rec, out := &recorder{}, &models.AcceptedResponse{}
	err := ResidencyDocumentVerification(ctx, rec, models.ResidencyDocumentVerificationParams{
		SelfieImage: img("selfie.jpg"), LivenessImages: liveness, Document: img("front.png"),
		DocumentBack: img("back.png"), Visa: img("visa.jpg"), Consent: consent, Country: "AE",
		IDType: "PASSPORT", UserDetails: userDetails, CallbackURL: str("https://cb"),
		PartnerParams: partner, Metadata: metadata, UserID: str("u3"),
	}, out)
	check(t, rec, err, out, shape{
		Method: "POST", Path: "/v3/residency_document_verification", UserIDHeader: "u3",
		Authenticated: true, NeedsPartnerIDHeader: true, BodyKind: BodyMultipart,
		Parts: []string{"country=AE", "id_type=PASSPORT", "selfie_image file=selfie.jpg",
			"liveness_images files=l1.jpg,l2.jpg", "document file=front.png", "document_back file=back.png",
			userDetailsJSON, consentJSON, "callback_url=https://cb", partnerJSON, metadataJSON, "visa file=visa.jpg"},
	})
}

func TestBiometricKYC(t *testing.T) {
	base := shape{Method: "POST", Path: "/v3/biometric_kyc", Authenticated: true, NeedsPartnerIDHeader: true, BodyKind: BodyMultipart}

	t.Run("all fields", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := BiometricKYC(ctx, rec, models.BiometricKYCParams{
			SelfieImage: img("selfie.jpg"), LivenessImages: liveness, Consent: consent, Country: "GH",
			IDType: "GHANA_CARD", IDNumber: "GHA-1", UserDetails: userDetails, CallbackURL: str("https://cb"),
			SandboxResult: flt(0.5), PartnerParams: partner, Metadata: metadata, UserID: str("u4"),
		}, out)
		want := base
		want.UserIDHeader = "u4"
		want.Parts = []string{"country=GH", "id_type=GHANA_CARD", "id_number=GHA-1", "selfie_image file=selfie.jpg",
			"liveness_images files=l1.jpg,l2.jpg", userDetailsJSON, consentJSON, "callback_url=https://cb",
			"sandbox_result=0.5", partnerJSON, metadataJSON}
		check(t, rec, err, out, want)
	})

	t.Run("enrolled image, no images", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := BiometricKYC(ctx, rec, models.BiometricKYCParams{
			Country: "GH", IDType: "GHANA_CARD", IDNumber: "GHA-1", UserDetails: userDetails, Consent: consent,
		}, out)
		want := base
		want.Parts = []string{"country=GH", "id_type=GHANA_CARD", "id_number=GHA-1", userDetailsJSON, consentJSON}
		check(t, rec, err, out, want)
	})
}

func TestRegistration(t *testing.T) {
	base := shape{Method: "POST", Path: "/v3/registration", Authenticated: true, BodyKind: BodyMultipart}

	t.Run("all fields", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := Registration(ctx, rec, models.RegistrationParams{
			SelfieImage: img("selfie.jpg"), LivenessImages: liveness, Consent: consent, UserDetails: userDetails,
			AllowNewEnroll: boolean(true), CallbackURL: str("https://cb"), SandboxResult: flt(1),
			PartnerParams: partner, Metadata: metadata, UserID: str("u5"),
		}, out)
		want := base
		want.UserIDHeader = "u5"
		want.Parts = []string{"selfie_image file=selfie.jpg", "liveness_images files=l1.jpg,l2.jpg", userDetailsJSON,
			consentJSON, "allow_new_enroll=true", "callback_url=https://cb", "sandbox_result=1", partnerJSON, metadataJSON}
		check(t, rec, err, out, want)
	})

	t.Run("required fields only", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := Registration(ctx, rec, models.RegistrationParams{
			SelfieImage: img("selfie.jpg"), LivenessImages: liveness, Consent: consent, UserDetails: userDetails,
		}, out)
		want := base
		want.Parts = []string{"selfie_image file=selfie.jpg", "liveness_images files=l1.jpg,l2.jpg", userDetailsJSON, consentJSON}
		check(t, rec, err, out, want)
	})
}

func TestAuthentication(t *testing.T) {
	base := shape{Method: "POST", Path: "/v3/authentication", Authenticated: true, BodyKind: BodyMultipart}

	t.Run("all fields", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := Authentication(ctx, rec, models.AuthenticationParams{
			UserID: "u6", SelfieImage: img("selfie.jpg"), LivenessImages: liveness, Consent: consent,
			UserDetails: userDetails, CallbackURL: str("https://cb"), SandboxResult: flt(0.25),
			PartnerParams: partner, Metadata: metadata,
		}, out)
		want := base
		want.Parts = []string{"user_id=u6", "selfie_image file=selfie.jpg", "liveness_images files=l1.jpg,l2.jpg",
			userDetailsJSON, consentJSON, "callback_url=https://cb", "sandbox_result=0.25", partnerJSON, metadataJSON}
		check(t, rec, err, out, want)
	})

	t.Run("required fields only", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := Authentication(ctx, rec, models.AuthenticationParams{
			UserID: "u6", SelfieImage: img("selfie.jpg"), LivenessImages: liveness, Consent: consent, UserDetails: userDetails,
		}, out)
		want := base
		want.Parts = []string{"user_id=u6", "selfie_image file=selfie.jpg", "liveness_images files=l1.jpg,l2.jpg",
			userDetailsJSON, consentJSON}
		check(t, rec, err, out, want)
	})
}

func TestCompare(t *testing.T) {
	base := shape{Method: "POST", Path: "/v3/compare", Authenticated: true, BodyKind: BodyMultipart}

	t.Run("all fields", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := Compare(ctx, rec, models.CompareParams{
			SelfieImage: img("selfie.jpg"), ComparisonImage: img("id.jpg"), ComparisonImageType: "ID_CARD",
			Consent: consent, UserDetails: userDetails, LivenessImages: liveness, AllowNewEnroll: boolean(false),
			UserID: str("u7"), CallbackURL: str("https://cb"), SandboxResult: flt(2),
			PartnerParams: partner, Metadata: metadata,
		}, out)
		want := base
		want.Parts = []string{"selfie_image file=selfie.jpg", "comparison_image file=id.jpg", "comparison_image_type=ID_CARD",
			userDetailsJSON, consentJSON, "liveness_images files=l1.jpg,l2.jpg", "allow_new_enroll=false",
			"user_id=u7", "callback_url=https://cb", "sandbox_result=2", partnerJSON, metadataJSON}
		check(t, rec, err, out, want)
	})

	t.Run("required fields only", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := Compare(ctx, rec, models.CompareParams{
			SelfieImage: img("selfie.jpg"), ComparisonImage: img("id.jpg"), ComparisonImageType: "SELFIE",
			Consent: consent, UserDetails: userDetails,
		}, out)
		want := base
		want.Parts = []string{"selfie_image file=selfie.jpg", "comparison_image file=id.jpg", "comparison_image_type=SELFIE",
			userDetailsJSON, consentJSON}
		check(t, rec, err, out, want)
	})
}

func TestRetrieveStatus(t *testing.T) {
	rec, out := &recorder{}, &models.JobStatus{}
	err := RetrieveStatus(ctx, rec, "job/1 2", out)
	check(t, rec, err, out, shape{
		Method: "GET", Path: "/v3/status/job%2F1%202", Authenticated: true, Idempotent: true, NotFoundReturnsBody: true,
	})
}

func TestReplay(t *testing.T) {
	t.Run("callback override", func(t *testing.T) {
		rec, out := &recorder{}, &models.ReplayCallbackResponse{}
		err := Replay(ctx, rec, "job-1", models.ReplayParams{CallbackURL: str("https://cb")}, out)
		check(t, rec, err, out, shape{
			Method: "POST", Path: "/v3/replay/job-1", Authenticated: true, BodyKind: BodyMultipart,
			Parts: []string{"callback_url=https://cb"},
		})
	})

	t.Run("no override sends no body", func(t *testing.T) {
		rec, out := &recorder{}, &models.ReplayCallbackResponse{}
		err := Replay(ctx, rec, "job-1", models.ReplayParams{}, out)
		check(t, rec, err, out, shape{Method: "POST", Path: "/v3/replay/job-1", Authenticated: true, BodyKind: BodyNone})
	})
}

func TestReportFraud(t *testing.T) {
	base := shape{Method: "POST", Path: "/v3/users/user%2F1/report_fraud", Authenticated: true, BodyKind: BodyMultipart}

	t.Run("all fields", func(t *testing.T) {
		rec, out := &recorder{}, &models.ReportUserFraudResponse{}
		err := ReportFraud(ctx, rec, "user/1", models.ReportFraudParams{
			IsFraud: true, ReportedBy: "ops@example.com", Reason: str("synthetic_identity"), Notes: str("seen twice"),
		}, out)
		want := base
		want.Parts = []string{"is_fraud=true", "reported_by=ops@example.com", "reason=synthetic_identity", "notes=seen twice"}
		check(t, rec, err, out, want)
	})

	t.Run("required fields only", func(t *testing.T) {
		rec, out := &recorder{}, &models.ReportUserFraudResponse{}
		err := ReportFraud(ctx, rec, "user/1", models.ReportFraudParams{ReportedBy: "ops@example.com"}, out)
		want := base
		want.Parts = []string{"is_fraud=false", "reported_by=ops@example.com"}
		check(t, rec, err, out, want)
	})
}

func TestServiceLookups(t *testing.T) {
	tests := []struct {
		name  string
		call  func(*recorder) (interface{}, error)
		path  string
		query string
		auth  bool
	}{
		{"bank codes with country", func(r *recorder) (interface{}, error) {
			out := &models.BankCodesResponse{}
			return out, BankCodes(ctx, r, models.BankCodesParams{Country: str("NG")}, out)
		}, "/v3/services/bank_codes", "country=NG", false},
		{"bank codes without query", func(r *recorder) (interface{}, error) {
			out := &models.BankCodesResponse{}
			return out, BankCodes(ctx, r, models.BankCodesParams{}, out)
		}, "/v3/services/bank_codes", "", false},
		{"supported id types with country", func(r *recorder) (interface{}, error) {
			out := &models.SupportedIDTypesResponse{}
			return out, SupportedIDTypes(ctx, r, models.SupportedIDTypesParams{Country: str("KE")}, out)
		}, "/v3/services/supported_id_types", "country=KE", false},
		{"supported id types without query", func(r *recorder) (interface{}, error) {
			out := &models.SupportedIDTypesResponse{}
			return out, SupportedIDTypes(ctx, r, models.SupportedIDTypesParams{}, out)
		}, "/v3/services/supported_id_types", "", false},
		{"supported documents with all filters", func(r *recorder) (interface{}, error) {
			out := &models.SupportedDocumentsResponse{}
			return out, SupportedDocuments(ctx, r, models.SupportedDocumentsParams{
				Continent: str("Africa"), CountryCode: str("ZA"), Locale: str("en"),
			}, out)
		}, "/v3/services/supported_documents", "continent=Africa&country_code=ZA&locale=en", false},
		{"supported documents without query", func(r *recorder) (interface{}, error) {
			out := &models.SupportedDocumentsResponse{}
			return out, SupportedDocuments(ctx, r, models.SupportedDocumentsParams{}, out)
		}, "/v3/services/supported_documents", "", false},
		{"id status", func(r *recorder) (interface{}, error) {
			out := &models.IDStatusResponse{}
			return out, IDStatus(ctx, r, models.IDStatusParams{Country: "NG", IDType: "NIN"}, out)
		}, "/v3/services/id_status", "country=NG&id_type=NIN", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recorder{}
			out, err := tt.call(rec)
			check(t, rec, err, out, shape{
				Method: "GET", Path: tt.path, Query: tt.query, Authenticated: tt.auth, Idempotent: true,
			})
		})
	}
}

func TestFloatFormatting(t *testing.T) {
	for in, want := range map[float64]string{0: "0", 1: "1", 0.75: "0.75", 1e-7: "0.0000001"} {
		if got := floatStr(in); got != want {
			t.Errorf("floatStr(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestOneTimeAML(t *testing.T) {
	base := shape{Method: "POST", Path: "/v3/aml", Authenticated: true, BodyKind: BodyMultipart}

	t.Run("all fields", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := OneTimeAML(ctx, rec, models.OneTimeAMLParams{
			Countries: []string{"NG", "KE"}, FullName: "John Doe", Consent: consent, UserDetails: userDetails,
			BirthYear: str("1990"), StrictMatch: boolean(false), Aliases: []string{"Johnny Doe"},
			SearchExistingUser: boolean(true), UserID: str("u1"), CallbackURL: str("https://cb"),
			PartnerParams: partner, Metadata: metadata,
		}, out)
		want := base
		want.Parts = []string{`countries json=["NG","KE"]`, "full_name=John Doe", userDetailsJSON, consentJSON,
			"birth_year=1990", "strict_match=false", `aliases json=["Johnny Doe"]`, "search_existing_user=true",
			"user_id=u1", "callback_url=https://cb", partnerJSON, metadataJSON}
		check(t, rec, err, out, want)
	})

	t.Run("required fields only", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := OneTimeAML(ctx, rec, models.OneTimeAMLParams{
			Countries: []string{"NG"}, FullName: "John Doe", Consent: consent, UserDetails: userDetails,
		}, out)
		want := base
		want.Parts = []string{`countries json=["NG"]`, "full_name=John Doe", userDetailsJSON, consentJSON}
		check(t, rec, err, out, want)
	})
}

func TestOneTimeAMLSync(t *testing.T) {
	params := models.OneTimeAMLParams{
		Countries: []string{"NG"}, FullName: "John Doe", Consent: consent, UserDetails: userDetails,
	}
	parts := []string{`countries json=["NG"]`, "full_name=John Doe", userDetailsJSON, consentJSON}

	t.Run("with idempotency key", func(t *testing.T) {
		rec, out := &recorder{}, &models.OneTimeAMLResult{}
		err := OneTimeAMLSync(ctx, rec, models.OneTimeAMLSyncParams{
			OneTimeAMLParams: params, IdempotencyKey: str("kuda-onboarding-8f14e45fceea167a"),
		}, out)
		if err != nil {
			t.Fatal(err)
		}
		if got := rec.req.Headers["SmileID-Idempotency-Key"]; got != "kuda-onboarding-8f14e45fceea167a" {
			t.Errorf("idempotency header = %q", got)
		}
		if !rec.req.Idempotent {
			t.Error("a keyed sync screening must be retryable")
		}
		if rec.req.Path != "/v3/aml/sync" || rec.req.Method != "POST" || !rec.req.Authenticated || rec.req.BodyKind != BodyMultipart {
			t.Errorf("request = %+v", rec.req)
		}
		if rec.out != out {
			t.Error("out not passed to Do")
		}
		takeResponseHeaders(t, rec.req, map[string]*string{
			"SmileID-Job-ID": &out.JobID, "User-ID": &out.UserID,
			"Response-Signature": &out.ResponseSignature, "Response-Timestamp": &out.ResponseTimestamp,
		})
	})

	t.Run("without idempotency key", func(t *testing.T) {
		rec, out := &recorder{}, &models.OneTimeAMLResult{}
		err := OneTimeAMLSync(ctx, rec, models.OneTimeAMLSyncParams{OneTimeAMLParams: params}, out)
		takeResponseHeaders(t, rec.req, map[string]*string{
			"SmileID-Job-ID": &out.JobID, "User-ID": &out.UserID,
			"Response-Signature": &out.ResponseSignature, "Response-Timestamp": &out.ResponseTimestamp,
		})
		want := shape{Method: "POST", Path: "/v3/aml/sync", Authenticated: true, BodyKind: BodyMultipart, Parts: parts}
		check(t, rec, err, out, want)
	})
}

func TestAMLMonitoring(t *testing.T) {
	base := shape{Method: "POST", Path: "/v3/aml/monitoring", Authenticated: true, BodyKind: BodyMultipart}

	t.Run("all fields", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := AMLMonitoring(ctx, rec, models.AMLMonitoringParams{
			Countries: []string{"NG", "GH"}, FullName: str("Jane Doe"), Consent: consent, UserDetails: userDetails,
			BirthYear: str("1985"), StrictMatch: boolean(true), Aliases: []string{"J. Doe"},
			SearchExistingUser: boolean(false), UserID: str("u1"), CallbackURL: str("https://cb"),
			PartnerParams: partner, Metadata: metadata,
		}, out)
		want := base
		want.Parts = []string{`countries json=["NG","GH"]`, "full_name=Jane Doe", userDetailsJSON, consentJSON,
			"birth_year=1985", "strict_match=true", `aliases json=["J. Doe"]`, "search_existing_user=false",
			"user_id=u1", "callback_url=https://cb", partnerJSON, metadataJSON}
		check(t, rec, err, out, want)
	})

	t.Run("required fields only", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := AMLMonitoring(ctx, rec, models.AMLMonitoringParams{
			Countries: []string{"NG"}, Consent: consent, UserDetails: userDetails,
		}, out)
		want := base
		want.Parts = []string{`countries json=["NG"]`, userDetailsJSON, consentJSON}
		check(t, rec, err, out, want)
	})
}

func TestAMLMonitoringNews(t *testing.T) {
	t.Run("all fields", func(t *testing.T) {
		rec, out := &recorder{}, &models.AMLMonitoringNewsResponse{}
		page, limit := 2, 50
		err := AMLMonitoringNews(ctx, rec, "job/1", "ref 001", models.AMLMonitoringNewsParams{
			NewsCategory: "financial_crime", Page: &page, Limit: &limit,
		}, out)
		check(t, rec, err, out, shape{
			Method: "GET", Path: "/v3/aml/monitoring/job%2F1/news/ref%20001",
			Query: "limit=50&news_category=financial_crime&page=2", Authenticated: true, Idempotent: true,
		})
	})

	t.Run("category only", func(t *testing.T) {
		rec, out := &recorder{}, &models.AMLMonitoringNewsResponse{}
		err := AMLMonitoringNews(ctx, rec, "j1", "r1", models.AMLMonitoringNewsParams{NewsCategory: "other"}, out)
		check(t, rec, err, out, shape{
			Method: "GET", Path: "/v3/aml/monitoring/j1/news/r1", Query: "news_category=other",
			Authenticated: true, Idempotent: true,
		})
	})
}

func TestPhoneVerification(t *testing.T) {
	base := shape{Method: "POST", Path: "/v3/phone", Authenticated: true, BodyKind: BodyMultipart}

	t.Run("all fields", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := PhoneVerification(ctx, rec, models.PhoneVerificationParams{
			Country: "GH", PhoneNumber: "+233201234567", Operator: str("MTN"), IDNumber: str("GHA-1"),
			Consent: consent, UserDetails: userDetails, UserID: str("u1"), CallbackURL: str("https://cb"),
			PartnerParams: partner, Metadata: metadata,
		}, out)
		want := base
		want.Parts = []string{"country=GH", "phone_number=+233201234567", "operator=MTN", "id_number=GHA-1",
			userDetailsJSON, consentJSON, "user_id=u1", "callback_url=https://cb", partnerJSON, metadataJSON}
		check(t, rec, err, out, want)
		if rec.req.Product != "phone_number_verification" || rec.req.Idempotent {
			t.Errorf("product = %q, idempotent = %v", rec.req.Product, rec.req.Idempotent)
		}
	})

	t.Run("operator without id number", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := PhoneVerification(ctx, rec, models.PhoneVerificationParams{
			Country: "GH", PhoneNumber: "+233201234567", Operator: str("VODAFONE"), Consent: consent, UserDetails: userDetails,
		}, out)
		want := base
		want.Parts = []string{"country=GH", "phone_number=+233201234567", "operator=VODAFONE", userDetailsJSON, consentJSON}
		check(t, rec, err, out, want)
	})

	t.Run("id number without operator", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := PhoneVerification(ctx, rec, models.PhoneVerificationParams{
			Country: "KE", PhoneNumber: "+254712345678", IDNumber: str("12345678"), Consent: consent, UserDetails: userDetails,
		}, out)
		want := base
		want.Parts = []string{"country=KE", "phone_number=+254712345678", "id_number=12345678", userDetailsJSON, consentJSON}
		check(t, rec, err, out, want)
	})

	t.Run("required fields only", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := PhoneVerification(ctx, rec, models.PhoneVerificationParams{
			Country: "NG", PhoneNumber: "+2348012345678", Consent: consent, UserDetails: userDetails,
		}, out)
		want := base
		want.Parts = []string{"country=NG", "phone_number=+2348012345678", userDetailsJSON, consentJSON}
		check(t, rec, err, out, want)
	})
}

func TestAddressVerification(t *testing.T) {
	base := shape{Method: "POST", Path: "/v3/address", Authenticated: true, BodyKind: BodyMultipart}

	t.Run("all fields", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := AddressVerification(ctx, rec, models.AddressVerificationParams{
			Address: "1 Marina Rd, Lagos", Country: "NG", IDNumber: str("1"), UtilityNumber: str("04123456789"),
			UtilityProvider: str("ikeja-electric"), UtilityType: str("PrePaid"), Consent: consent,
			UserDetails: userDetails, UserID: str("u1"), CallbackURL: str("https://cb"),
			PartnerParams: partner, Metadata: metadata,
		}, out)
		want := base
		want.Parts = []string{"address=1 Marina Rd, Lagos", "country=NG", "id_number=1", "utility_number=04123456789",
			"utility_provider=ikeja-electric", "utility_type=PrePaid", userDetailsJSON, consentJSON,
			"user_id=u1", "callback_url=https://cb", partnerJSON, metadataJSON}
		check(t, rec, err, out, want)
		if rec.req.Product != "address_verification" || rec.req.Idempotent {
			t.Errorf("product = %q, idempotent = %v", rec.req.Product, rec.req.Idempotent)
		}
	})

	t.Run("utility account without id number", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := AddressVerification(ctx, rec, models.AddressVerificationParams{
			Address: "3 Pier Rd", Country: "NG", UtilityNumber: str("04123456789"), UtilityProvider: str("eko-electric"),
			UtilityType: str("PostPaid"), Consent: consent, UserDetails: userDetails,
		}, out)
		want := base
		want.Parts = []string{"address=3 Pier Rd", "country=NG", "utility_number=04123456789",
			"utility_provider=eko-electric", "utility_type=PostPaid", userDetailsJSON, consentJSON}
		check(t, rec, err, out, want)
	})

	t.Run("required fields only", func(t *testing.T) {
		rec, out := &recorder{}, &models.AcceptedResponse{}
		err := AddressVerification(ctx, rec, models.AddressVerificationParams{
			Address: "2 Main St", Country: "ZA", IDNumber: str("8001015009087"), Consent: consent, UserDetails: userDetails,
		}, out)
		want := base
		want.Parts = []string{"address=2 Main St", "country=ZA", "id_number=8001015009087", userDetailsJSON, consentJSON}
		check(t, rec, err, out, want)
	})
}

var smileLinkTypes = []models.SmileLinkIDType{{Country: "NG", IDType: "NIN", VerificationMethod: "enhanced_kyc"}}

const smileLinkTypesJSON = `id_types json=[{"country":"NG","id_type":"NIN","verification_method":"enhanced_kyc"}]`

func TestCreateSmileLink(t *testing.T) {
	base := shape{Method: "POST", Path: "/v3/smile_links", Authenticated: true, BodyKind: BodyMultipart}

	t.Run("all fields", func(t *testing.T) {
		rec, out := &recorder{}, &models.SmileLinkCreated{}
		err := CreateSmileLink(ctx, rec, models.CreateSmileLinkParams{
			IDTypes: smileLinkTypes, Name: "Onboarding", CompanyName: str("Acme"), Description: str("d"),
			NoticePrivacyPolicyURL: "https://example.com/p", IsSingleUse: true, CallbackURL: str("https://cb"),
			ExpiresAt: str("2027-01-01T00:00:00Z"), LogoURL: str("https://logo"), RedirectURL: str("https://redirect"),
			ThemeColor: str("#0a0a0a"), PartnerParams: map[string]interface{}{"k": "v"}, UserID: str("u1"),
		}, out)
		want := base
		want.Parts = []string{smileLinkTypesJSON, "name=Onboarding", "company_name=Acme", "description=d",
			"notice_privacy_policy_url=https://example.com/p", "is_single_use=true", "callback_url=https://cb",
			"expires_at=2027-01-01T00:00:00Z", "logo_url=https://logo", "redirect_url=https://redirect",
			"theme_color=#0a0a0a", `partner_params json={"k":"v"}`, "user_id=u1"}
		check(t, rec, err, out, want)
		if rec.req.Product != "" || rec.req.Idempotent {
			t.Errorf("product = %q, idempotent = %v", rec.req.Product, rec.req.Idempotent)
		}
	})

	t.Run("required fields only", func(t *testing.T) {
		rec, out := &recorder{}, &models.SmileLinkCreated{}
		err := CreateSmileLink(ctx, rec, models.CreateSmileLinkParams{
			IDTypes: smileLinkTypes, Name: "Onboarding", NoticePrivacyPolicyURL: "https://example.com/p",
		}, out)
		want := base
		want.Parts = []string{smileLinkTypesJSON, "name=Onboarding", "notice_privacy_policy_url=https://example.com/p", "is_single_use=false"}
		check(t, rec, err, out, want)
	})
}

func TestListSmileLinks(t *testing.T) {
	t.Run("all fields", func(t *testing.T) {
		rec, out := &recorder{}, &models.SmileLinkListResponse{}
		limit, page := 50, 2
		err := ListSmileLinks(ctx, rec, models.ListSmileLinksParams{Limit: &limit, Page: &page, SingleUse: boolean(false)}, out)
		check(t, rec, err, out, shape{Method: "GET", Path: "/v3/smile_links", Query: "limit=50&page=2&single_use=false", Authenticated: true, Idempotent: true})
		if rec.req.Product != "" {
			t.Errorf("product = %q", rec.req.Product)
		}
	})

	t.Run("no fields", func(t *testing.T) {
		rec, out := &recorder{}, &models.SmileLinkListResponse{}
		err := ListSmileLinks(ctx, rec, models.ListSmileLinksParams{}, out)
		check(t, rec, err, out, shape{Method: "GET", Path: "/v3/smile_links", Authenticated: true, Idempotent: true})
	})
}

func TestGetSmileLink(t *testing.T) {
	rec, out := &recorder{}, &models.SmileLink{}
	err := GetSmileLink(ctx, rec, "a/b", out)
	check(t, rec, err, out, shape{Method: "GET", Path: "/v3/smile_links/a%2Fb", Authenticated: true, Idempotent: true})
	if rec.req.Product != "" {
		t.Errorf("product = %q", rec.req.Product)
	}
}

func TestUpdateSmileLink(t *testing.T) {
	base := shape{Method: "PUT", Path: "/v3/smile_links/ref-1", Authenticated: true, BodyKind: BodyMultipart}

	t.Run("all fields", func(t *testing.T) {
		rec, out := &recorder{}, &models.SmileLink{}
		err := UpdateSmileLink(ctx, rec, "ref-1", models.UpdateSmileLinkParams{IDTypes: smileLinkTypes, IsDisabled: boolean(true)}, out)
		want := base
		want.Parts = []string{smileLinkTypesJSON, "is_disabled=true"}
		check(t, rec, err, out, want)
		if rec.req.Product != "" || rec.req.Idempotent {
			t.Errorf("product = %q, idempotent = %v", rec.req.Product, rec.req.Idempotent)
		}
	})

	t.Run("id_types only", func(t *testing.T) {
		rec, out := &recorder{}, &models.SmileLink{}
		err := UpdateSmileLink(ctx, rec, "ref-1", models.UpdateSmileLinkParams{IDTypes: smileLinkTypes}, out)
		want := base
		want.Parts = []string{smileLinkTypesJSON}
		check(t, rec, err, out, want)
	})

	t.Run("is_disabled only", func(t *testing.T) {
		rec, out := &recorder{}, &models.SmileLink{}
		err := UpdateSmileLink(ctx, rec, "ref-1", models.UpdateSmileLinkParams{IsDisabled: boolean(false)}, out)
		want := base
		want.Parts = []string{"is_disabled=false"}
		check(t, rec, err, out, want)
	})
}

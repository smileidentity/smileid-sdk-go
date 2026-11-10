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

package usesmileid

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/smileidentity/smileid-sdk-go/v13/generated/operations"
)

// These tests keep spec-coverage.json honest against the code. CI checks the
// file against the spec; here every supported method must exist and the
// supported paths must be exactly the paths the SDK sends.

type coverageEntry struct {
	Method      string  `json:"method"`
	Path        string  `json:"path"`
	Unsupported *string `json:"unsupported"`
}

func supportedCoverage(t *testing.T) map[string]coverageEntry {
	t.Helper()
	b, err := os.ReadFile("spec-coverage.json")
	if err != nil {
		t.Fatalf("read spec-coverage.json: %v", err)
	}
	var file struct {
		Operations map[string]coverageEntry `json:"operations"`
	}
	if err := json.Unmarshal(b, &file); err != nil {
		t.Fatalf("parse spec-coverage.json: %v", err)
	}
	supported := map[string]coverageEntry{}
	for id, e := range file.Operations {
		if e.Unsupported == nil {
			supported[id] = e
		}
	}
	if len(supported) == 0 {
		t.Fatal("spec-coverage.json lists no supported operations")
	}
	return supported
}

// internalMethods are the SDK functions that call an operation callers never
// call themselves. Each value is a method expression, so removing the method
// stops this file compiling.
var internalMethods = map[string]interface{}{
	// POST /v3/token: the transport mints a token before every
	// authenticated request.
	"transport.mintToken": (*transport).mintToken,
}

// resolveClientMethod follows a "Client.Resource.Method" chain through
// exported fields of a real client and returns the method.
func resolveClientMethod(c *Client, chain string) (reflect.Value, error) {
	parts := strings.Split(chain, ".")
	if len(parts) < 2 || parts[0] != "Client" {
		return reflect.Value{}, fmt.Errorf("%q must start with Client. or be listed in internalMethods", chain)
	}
	v := reflect.ValueOf(c)
	for _, name := range parts[1 : len(parts)-1] {
		sf, ok := reflect.Indirect(v).Type().FieldByName(name)
		if !ok || !sf.IsExported() {
			return reflect.Value{}, fmt.Errorf("%q: no exported field %s", chain, name)
		}
		v = reflect.Indirect(v).FieldByIndex(sf.Index)
		if v.Kind() == reflect.Pointer && v.IsNil() {
			return reflect.Value{}, fmt.Errorf("%q: field %s is nil on a new client", chain, name)
		}
	}
	m := v.MethodByName(parts[len(parts)-1])
	if !m.IsValid() {
		return reflect.Value{}, fmt.Errorf("%q: no method %s", chain, parts[len(parts)-1])
	}
	return m, nil
}

func TestSpecCoverageMethodsExist(t *testing.T) {
	c, err := NewClient(Config{PartnerID: "1234", APIKey: "test-key"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	used := map[string]bool{}
	for id, e := range supportedCoverage(t) {
		if fn, ok := internalMethods[e.Method]; ok {
			used[e.Method] = true
			if reflect.ValueOf(fn).Kind() != reflect.Func {
				t.Errorf("%s: internalMethods[%q] is not a function", id, e.Method)
			}
			continue
		}
		m, err := resolveClientMethod(c, e.Method)
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		if m.Kind() != reflect.Func {
			t.Errorf("%s: %q is not callable", id, e.Method)
		}
	}
	for name := range internalMethods {
		if !used[name] {
			t.Errorf("internalMethods[%q] is not named by any supported operation in spec-coverage.json", name)
		}
	}
}

func TestResolveClientMethodRejectsBadChains(t *testing.T) {
	c, err := NewClient(Config{PartnerID: "1234", APIKey: "test-key"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	for _, chain := range []string{"BasicKYC.Verify", "Client", "Client.Nope.Verify", "Client.cfg.Verify", "Client.BasicKYC.Nope"} {
		if _, err := resolveClientMethod(c, chain); err == nil {
			t.Errorf("%q resolved, want an error", chain)
		}
	}
	c.AML = nil
	if _, err := resolveClientMethod(c, "Client.AML.Screen"); err == nil {
		t.Error("a nil resource resolved, want an error")
	}
}

type pathRecorder struct{ paths map[string]bool }

func (r *pathRecorder) Do(_ context.Context, req *operations.Request, _ interface{}) error {
	p, err := url.PathUnescape(req.Path)
	if err != nil {
		return err
	}
	r.paths[req.Method+" "+p] = true
	return nil
}

// operationCalls calls each function in generated/operations once, passing
// each path parameter as its spec placeholder, such as "{job_id}", so the
// recorded path reads as the spec writes it.
var operationCalls = map[string]func(context.Context, operations.Doer) error{
	"EnhancedKYC": func(ctx context.Context, d operations.Doer) error {
		return operations.EnhancedKYC(ctx, d, EnhancedKYCParams{}, &AcceptedResponse{})
	},
	"EnhancedKYCSync": func(ctx context.Context, d operations.Doer) error {
		return operations.EnhancedKYCSync(ctx, d, EnhancedKYCParams{}, &EnhancedKYCResult{})
	},
	"BasicKYC": func(ctx context.Context, d operations.Doer) error {
		return operations.BasicKYC(ctx, d, BasicKYCParams{}, &AcceptedResponse{})
	},
	"BasicKYCSync": func(ctx context.Context, d operations.Doer) error {
		return operations.BasicKYCSync(ctx, d, BasicKYCParams{}, &BasicKYCResult{})
	},
	"DocumentVerification": func(ctx context.Context, d operations.Doer) error {
		return operations.DocumentVerification(ctx, d, DocumentVerificationParams{}, &AcceptedResponse{})
	},
	"EnhancedDocumentVerification": func(ctx context.Context, d operations.Doer) error {
		return operations.EnhancedDocumentVerification(ctx, d, DocumentVerificationParams{}, &AcceptedResponse{})
	},
	"ResidencyDocumentVerification": func(ctx context.Context, d operations.Doer) error {
		return operations.ResidencyDocumentVerification(ctx, d, ResidencyDocumentVerificationParams{}, &AcceptedResponse{})
	},
	"BiometricKYC": func(ctx context.Context, d operations.Doer) error {
		return operations.BiometricKYC(ctx, d, BiometricKYCParams{}, &AcceptedResponse{})
	},
	"Registration": func(ctx context.Context, d operations.Doer) error {
		return operations.Registration(ctx, d, RegistrationParams{}, &AcceptedResponse{})
	},
	"Authentication": func(ctx context.Context, d operations.Doer) error {
		return operations.Authentication(ctx, d, AuthenticationParams{}, &AcceptedResponse{})
	},
	"Compare": func(ctx context.Context, d operations.Doer) error {
		return operations.Compare(ctx, d, CompareParams{}, &AcceptedResponse{})
	},
	"RetrieveStatus": func(ctx context.Context, d operations.Doer) error {
		return operations.RetrieveStatus(ctx, d, "{jobId}", &JobStatus{})
	},
	"Replay": func(ctx context.Context, d operations.Doer) error {
		return operations.Replay(ctx, d, "{job_id}", ReplayParams{}, &ReplayCallbackResponse{})
	},
	"ReportFraud": func(ctx context.Context, d operations.Doer) error {
		return operations.ReportFraud(ctx, d, "{user_id}", ReportFraudParams{}, &ReportUserFraudResponse{})
	},
	"BankCodes": func(ctx context.Context, d operations.Doer) error {
		return operations.BankCodes(ctx, d, BankCodesParams{}, &BankCodesResponse{})
	},
	"SupportedIDTypes": func(ctx context.Context, d operations.Doer) error {
		return operations.SupportedIDTypes(ctx, d, SupportedIDTypesParams{}, &SupportedIDTypesResponse{})
	},
	"SupportedDocuments": func(ctx context.Context, d operations.Doer) error {
		return operations.SupportedDocuments(ctx, d, SupportedDocumentsParams{}, &SupportedDocumentsResponse{})
	},
	"IDStatus": func(ctx context.Context, d operations.Doer) error {
		return operations.IDStatus(ctx, d, IDStatusParams{}, &IDStatusResponse{})
	},
	"OneTimeAML": func(ctx context.Context, d operations.Doer) error {
		return operations.OneTimeAML(ctx, d, OneTimeAMLParams{}, &AcceptedResponse{})
	},
	"OneTimeAMLSync": func(ctx context.Context, d operations.Doer) error {
		return operations.OneTimeAMLSync(ctx, d, OneTimeAMLSyncParams{}, &OneTimeAMLResult{})
	},
	"AMLMonitoring": func(ctx context.Context, d operations.Doer) error {
		return operations.AMLMonitoring(ctx, d, AMLMonitoringParams{}, &AcceptedResponse{})
	},
	"AMLMonitoringNews": func(ctx context.Context, d operations.Doer) error {
		return operations.AMLMonitoringNews(ctx, d, "{job_id}", "{reference}", AMLMonitoringNewsParams{}, &AMLMonitoringNewsResponse{})
	},
	"PhoneVerification": func(ctx context.Context, d operations.Doer) error {
		return operations.PhoneVerification(ctx, d, PhoneVerificationParams{}, &AcceptedResponse{})
	},
	"AddressVerification": func(ctx context.Context, d operations.Doer) error {
		return operations.AddressVerification(ctx, d, AddressVerificationParams{}, &AcceptedResponse{})
	},
	"CreateSmileLink": func(ctx context.Context, d operations.Doer) error {
		return operations.CreateSmileLink(ctx, d, CreateSmileLinkParams{}, &SmileLinkCreated{})
	},
	"ListSmileLinks": func(ctx context.Context, d operations.Doer) error {
		return operations.ListSmileLinks(ctx, d, ListSmileLinksParams{}, &SmileLinkListResponse{})
	},
	"GetSmileLink": func(ctx context.Context, d operations.Doer) error {
		return operations.GetSmileLink(ctx, d, "{refId}", &SmileLink{})
	},
	"UpdateSmileLink": func(ctx context.Context, d operations.Doer) error {
		return operations.UpdateSmileLink(ctx, d, "{refId}", UpdateSmileLinkParams{}, &SmileLink{})
	},
}

// operationFuncs lists the exported functions in generated/operations that
// send a request: those taking an operations.Doer.
func operationFuncs(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("generated", "operations", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !fn.Name.IsExported() {
				continue
			}
			for _, field := range fn.Type.Params.List {
				if id, ok := field.Type.(*ast.Ident); ok && id.Name == "Doer" {
					names = append(names, fn.Name.Name)
					break
				}
			}
		}
	}
	sort.Strings(names)
	return names
}

func TestSpecCoveragePathsMatchOperations(t *testing.T) {
	funcs := operationFuncs(t)
	if len(funcs) == 0 {
		t.Fatal("found no operation functions in generated/operations")
	}
	for _, name := range funcs {
		if _, ok := operationCalls[name]; !ok {
			t.Errorf("operations.%s is not in operationCalls: add it, and add its operation to spec-coverage.json", name)
		}
	}
	listed := map[string]bool{}
	for _, name := range funcs {
		listed[name] = true
	}
	for name := range operationCalls {
		if !listed[name] {
			t.Errorf("operationCalls[%q] is not a function in generated/operations", name)
		}
	}

	rec := &pathRecorder{paths: map[string]bool{}}
	for name, call := range operationCalls {
		if err := call(context.Background(), rec); err != nil {
			t.Fatalf("operations.%s: %v", name, err)
		}
	}

	// The token request is built by the transport, not generated/operations.
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		rec.paths[r.Method+" "+r.URL.Path] = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"t"}`))
	})
	if _, err := c.transport.mintToken(context.Background(), &operations.Request{}); err != nil {
		t.Fatalf("mintToken: %v", err)
	}

	supported := map[string]string{}
	for id, e := range supportedCoverage(t) {
		supported[e.Path] = id
	}
	for p := range rec.paths {
		if _, ok := supported[p]; !ok {
			t.Errorf("the SDK sends %s, but spec-coverage.json has no supported operation with that path", p)
		}
	}
	for p, id := range supported {
		if !rec.paths[p] {
			t.Errorf("spec-coverage.json says %s (%s) is supported, but the SDK never sends it", id, p)
		}
	}
}

package versions

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// openAPIServer serves an OpenAPI spec with the given API version, and fails the
// test if the request lacks the bearer token.
func openAPIServer(t *testing.T, apiVersion string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openapi.json" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("expected bearer token, got %q", got)
		}
		fmt.Fprintf(w, `{"info": {"version": %q}}`, apiVersion)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func setDetectEnv(t *testing.T, pin, host, token string) {
	t.Helper()
	t.Setenv("BLASTSHIELD_API_VERSION", pin)
	t.Setenv("BLASTSHIELD_HOST", host)
	t.Setenv("BLASTSHIELD_TOKEN", token)
}

func registerTestVersions() {
	resetRegistry()
	Register("1.13.0", &mockVersionedProvider{version: "1.13.0"})
	Register("2.0.0", &mockVersionedProvider{version: "2.0.0"})
}

func TestDetect_Pinned(t *testing.T) {
	registerTestVersions()
	// The pin takes precedence over detection
	srv := openAPIServer(t, "2.0.0")
	setDetectEnv(t, "1.13", srv.URL, "test-token")

	sel := Detect()
	if sel.Err != nil {
		t.Fatalf("unexpected error: %v", sel.Err)
	}
	if sel.Version != "1.13.0" || sel.Source != SourcePinned {
		t.Errorf("expected pinned 1.13.0, got %s (source %d)", sel.Version, sel.Source)
	}
	if sel.Provider == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestDetect_PinnedBetweenVersions(t *testing.T) {
	registerTestVersions()
	setDetectEnv(t, "1.20.0", "", "")

	sel := Detect()
	if sel.Version != "1.13.0" || sel.Source != SourcePinned {
		t.Errorf("expected pinned 1.13.0, got %s (source %d)", sel.Version, sel.Source)
	}
}

func TestDetect_PinnedUnsupported(t *testing.T) {
	registerTestVersions()
	setDetectEnv(t, "1.12", "", "")

	sel := Detect()
	if sel.Err == nil {
		t.Fatal("expected error for a pin below all supported versions")
	}
	if sel.Version != "2.0.0" || sel.Source != SourceLatest {
		t.Errorf("expected latest 2.0.0, got %s (source %d)", sel.Version, sel.Source)
	}
}

func TestDetect_FromServer(t *testing.T) {
	registerTestVersions()
	srv := openAPIServer(t, "1.13.4")
	setDetectEnv(t, "", srv.URL, "test-token")

	sel := Detect()
	if sel.Version != "1.13.0" || sel.Source != SourceDetected {
		t.Errorf("expected detected 1.13.0, got %s (source %d)", sel.Version, sel.Source)
	}
	if sel.Host != srv.URL {
		t.Errorf("expected host %s, got %s", srv.URL, sel.Host)
	}
}

func TestDetect_ServerUnreachable(t *testing.T) {
	registerTestVersions()
	srv := openAPIServer(t, "1.13.0")
	srv.Close()
	setDetectEnv(t, "", srv.URL, "test-token")

	sel := Detect()
	if sel.Version != "2.0.0" || sel.Source != SourceLatest {
		t.Errorf("expected latest 2.0.0, got %s (source %d)", sel.Version, sel.Source)
	}
}

func TestDetect_NoEnvironment(t *testing.T) {
	registerTestVersions()
	setDetectEnv(t, "", "", "")

	sel := Detect()
	if sel.Version != "2.0.0" || sel.Source != SourceLatest {
		t.Errorf("expected latest 2.0.0, got %s (source %d)", sel.Version, sel.Source)
	}
}

func TestFetchAPIVersion(t *testing.T) {
	srv := openAPIServer(t, "2.0.0")

	ver, err := FetchAPIVersion(srv.URL, "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "2.0.0" {
		t.Errorf("expected 2.0.0, got %s", ver)
	}
}

func TestFetchAPIVersion_Errors(t *testing.T) {
	tests := map[string]http.HandlerFunc{
		"status":          func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) },
		"invalid json":    func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "not json") },
		"missing version": func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"info": {}}`) },
	}
	for name, handler := range tests {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(handler)
			defer srv.Close()

			if _, err := FetchAPIVersion(srv.URL, "test-token"); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

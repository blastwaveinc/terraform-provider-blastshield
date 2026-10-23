// Copyright 2026 BlastWave, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/blastwaveinc/terraform-provider-blastshield/internal/provider/versions"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// The compiled-in versions are registered by the versionimports import in
// provider_test.go.

func TestAPIVersionMismatch(t *testing.T) {
	tests := map[string]struct {
		sel           versions.Selection
		serverVersion string
		severity      diag.Severity // 0 for no diagnostics
		summary       string
	}{
		"matching version": {
			sel:           versions.Selection{Version: "2.0.0", Source: versions.SourceLatest},
			serverVersion: "2.0.0",
		},
		"newer server within the version": {
			sel:           versions.Selection{Version: "2.0.0", Source: versions.SourceLatest},
			serverVersion: "2.1.3",
		},
		"pinned mismatch warns": {
			sel:           versions.Selection{Version: "1.13.0", Source: versions.SourcePinned},
			serverVersion: "2.0.0",
			severity:      diag.SeverityWarning,
			summary:       "Pinned Blastshield API Version Differs From the Server",
		},
		"unpinned mismatch errors": {
			sel:           versions.Selection{Version: "2.0.0", Source: versions.SourceLatest},
			serverVersion: "1.13.0",
			severity:      diag.SeverityError,
			summary:       "Blastshield API Version Mismatch",
		},
		"detected from another host errors": {
			sel:           versions.Selection{Version: "2.0.0", Source: versions.SourceDetected, Host: "https://other.example.com"},
			serverVersion: "1.13.0",
			severity:      diag.SeverityError,
			summary:       "Blastshield API Version Mismatch",
		},
		"unsupported server errors": {
			sel:           versions.Selection{Version: "2.0.0", Source: versions.SourceLatest},
			serverVersion: "0.0.0",
			severity:      diag.SeverityError,
			summary:       "Unsupported Blastshield API Version",
		},
		"unsupported server warns when pinned": {
			sel:           versions.Selection{Version: "2.0.0", Source: versions.SourcePinned},
			serverVersion: "0.0.0",
			severity:      diag.SeverityWarning,
			summary:       "Unsupported Blastshield API Version",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			diags := apiVersionMismatch(tt.sel, "https://blastshield.example.com", tt.serverVersion)
			checkSingleDiagnostic(t, diags, tt.severity, tt.summary)
		})
	}
}

func TestAPIVersionMismatch_ErrorNamesFix(t *testing.T) {
	sel := versions.Selection{Version: "2.0.0", Source: versions.SourceLatest}
	diags := apiVersionMismatch(sel, "https://blastshield.example.com", "1.13.2")
	if len(diags) != 1 {
		t.Fatalf("expected one diagnostic, got %v", diags)
	}
	detail := diags[0].Detail()
	for _, want := range []string{"BLASTSHIELD_HOST", "BLASTSHIELD_API_VERSION=1.13.0"} {
		if !strings.Contains(detail, want) {
			t.Errorf("expected detail to mention %q, got: %s", want, detail)
		}
	}
}

func TestCheckAPIVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"info": {"version": "1.13.0"}}`)
	}))
	defer srv.Close()
	unreachable := httptest.NewServer(http.NotFoundHandler())
	unreachable.Close()

	tests := map[string]struct {
		sel      versions.Selection
		host     string
		severity diag.Severity
		summary  string
	}{
		"detected from the same host skips the check": {
			// The detected version doesn't match the server, which shows it isn't fetched again
			sel:  versions.Selection{Version: "2.0.0", Source: versions.SourceDetected, Host: srv.URL},
			host: srv.URL,
		},
		"configured host is checked": {
			sel:      versions.Selection{Version: "2.0.0", Source: versions.SourceLatest},
			host:     srv.URL,
			severity: diag.SeverityError,
			summary:  "Blastshield API Version Mismatch",
		},
		"unreachable server warns": {
			sel:      versions.Selection{Version: "2.0.0", Source: versions.SourceLatest},
			host:     unreachable.URL,
			severity: diag.SeverityWarning,
			summary:  "Unable to Verify the Blastshield API Version",
		},
		"unsupported pin errors": {
			sel:      versions.Selection{Version: "2.0.0", Source: versions.SourceLatest, Err: errors.New("BLASTSHIELD_API_VERSION=1.12: below all supported versions")},
			host:     srv.URL,
			severity: diag.SeverityError,
			summary:  "Unsupported BLASTSHIELD_API_VERSION",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			p := &BlastshieldProvider{sel: tt.sel}
			diags := p.checkAPIVersion(tt.host, "test-token")
			checkSingleDiagnostic(t, diags, tt.severity, tt.summary)
		})
	}
}

// checkSingleDiagnostic checks that diags is empty when severity is 0, or holds a
// single diagnostic with the given severity and summary.
func checkSingleDiagnostic(t *testing.T, diags diag.Diagnostics, severity diag.Severity, summary string) {
	t.Helper()
	if severity == 0 {
		if len(diags) != 0 {
			t.Fatalf("expected no diagnostics, got %v", diags)
		}
		return
	}
	if len(diags) != 1 {
		t.Fatalf("expected one diagnostic, got %v", diags)
	}
	if diags[0].Severity() != severity || diags[0].Summary() != summary {
		t.Errorf("expected %s %q, got %s %q: %s", severity, summary, diags[0].Severity(), diags[0].Summary(), diags[0].Detail())
	}
}

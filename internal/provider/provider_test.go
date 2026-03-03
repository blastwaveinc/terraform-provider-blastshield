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
	"os"
	"testing"

	"github.com/blastwaveinc/terraform-provider-blastshield/internal/provider/versions"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	_ "github.com/blastwaveinc/terraform-provider-blastshield/internal/provider/versionimports"
)

const (
	// Default test configuration - can be overridden by environment variables
	defaultTestHost  = "http://localhost:4999"
	defaultTestToken = "dev"

	// Tag used to identify test-created resources for cleanup
	TestTag = "blastshield_tf_testing_entity"
)

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
func testVersionedProvider() versions.VersionedProvider {
	vp, _ := versions.LatestVersion()
	return vp
}

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"blastshield": providerserver.NewProtocol6WithError(New("test", testVersionedProvider())()),
}

func testAccPreCheck(t *testing.T) {
	// Ensure provider requirements are met
	if os.Getenv("BLASTSHIELD_HOST") == "" {
		os.Setenv("BLASTSHIELD_HOST", defaultTestHost)
	}
	if os.Getenv("BLASTSHIELD_TOKEN") == "" {
		os.Setenv("BLASTSHIELD_TOKEN", defaultTestToken)
	}
}

// Helper to get provider config block for tests
func testAccProviderConfig() string {
	return `
provider "blastshield" {
  # Configured via environment variables
}
`
}

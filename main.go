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

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/blastwaveinc/terraform-provider-blastshield/internal/provider"
	"github.com/blastwaveinc/terraform-provider-blastshield/internal/provider/versions"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	// Import all version packages so their init() functions register them.
	_ "github.com/blastwaveinc/terraform-provider-blastshield/internal/provider/versionimports"
)

var (
	version string = "dev"
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/blastwave/blastshield",
		Debug:   debug,
	}

	// Detect API version before starting the provider server.
	// Resources()/DataSources() are called before Configure(), so we need
	// to select the right version package upfront.
	vp := detectVersion()

	err := providerserver.Serve(context.Background(), provider.New(version, vp), opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}

func detectVersion() versions.VersionedProvider {
	host := os.Getenv("BLASTSHIELD_HOST")
	token := os.Getenv("BLASTSHIELD_TOKEN")

	if host != "" && token != "" {
		serverVersion, err := fetchAPIVersion(host, token)
		if err != nil {
			log.Printf("[WARN] Could not fetch API version from %s: %v, using latest compiled version", host, err)
			vp, ver := versions.LatestVersion()
			if vp != nil {
				log.Printf("[INFO] Using latest API version: %s", ver)
			}
			return vp
		}

		vp, ver, err := versions.SelectVersion(serverVersion)
		if err != nil {
			log.Fatalf("[ERROR] API version %s not supported: %v", serverVersion, err)
		}
		log.Printf("[INFO] Server API version: %s, using provider version: %s", serverVersion, ver)
		return vp
	}

	// No credentials available (e.g., terraform validate) — use latest version
	vp, ver := versions.LatestVersion()
	if vp != nil {
		log.Printf("[INFO] No BLASTSHIELD_HOST/TOKEN set, using latest API version: %s", ver)
	}
	return vp
}

func fetchAPIVersion(host, token string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", host+"/openapi.json", nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching openapi.json: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openapi.json returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	var spec struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
	}
	if err := json.Unmarshal(body, &spec); err != nil {
		return "", fmt.Errorf("parsing openapi.json: %w", err)
	}

	if spec.Info.Version == "" {
		return "", fmt.Errorf("openapi.json missing info.version")
	}

	return spec.Info.Version, nil
}

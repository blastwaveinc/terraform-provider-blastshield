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
	"flag"
	"log"

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
		Address: "registry.terraform.io/blastwaveinc/blastshield",
		Debug:   debug,
	}

	// Detect API version before starting the provider server.
	// Resources()/DataSources() are called before Configure(), so we need
	// to select the right version package upfront.
	sel := versions.Detect()

	err := providerserver.Serve(context.Background(), provider.New(version, sel), opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}

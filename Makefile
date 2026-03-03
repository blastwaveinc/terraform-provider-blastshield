HOSTNAME=registry.terraform.io
NAMESPACE=blastwaveinc
NAME=blastshield
BINARY=terraform-provider-${NAME}
VERSION=$(shell git describe --tags --always | sed 's/^v//')
OS_ARCH=$(shell go env GOOS)_$(shell go env GOARCH)

# Test configuration - override these via environment variables
BLASTSHIELD_HOST ?= http://localhost:4999
BLASTSHIELD_TOKEN ?= dev

default: install

build: generate
	go build -o ${BINARY}

release:
	GOOS=darwin GOARCH=amd64 go build -o ./bin/${BINARY}_${VERSION}_darwin_amd64
	GOOS=darwin GOARCH=arm64 go build -o ./bin/${BINARY}_${VERSION}_darwin_arm64
	GOOS=linux GOARCH=amd64 go build -o ./bin/${BINARY}_${VERSION}_linux_amd64
	GOOS=linux GOARCH=arm64 go build -o ./bin/${BINARY}_${VERSION}_linux_arm64
	GOOS=windows GOARCH=amd64 go build -o ./bin/${BINARY}_${VERSION}_windows_amd64.exe

install: build
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}
	mv ${BINARY} ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}

# Run unit tests (no API required)
test:
	go test -v ./...

# Run all acceptance tests
testacc:
	@echo "Running acceptance tests..."
	@echo "API: $(BLASTSHIELD_HOST)"
	@echo ""
	BLASTSHIELD_HOST=$(BLASTSHIELD_HOST) BLASTSHIELD_TOKEN=$(BLASTSHIELD_TOKEN) \
		TF_ACC=1 go test -v ./internal/provider/... -timeout 120m -count=1

# Run acceptance tests for a specific resource (e.g., make testacc-Node)
testacc-%:
	@echo "Running tests for: $*"
	@echo "Using API: $(BLASTSHIELD_HOST)"
	@echo ""
	BLASTSHIELD_HOST=$(BLASTSHIELD_HOST) BLASTSHIELD_TOKEN=$(BLASTSHIELD_TOKEN) \
		TF_ACC=1 go test ./internal/provider/... -run 'TestAcc$*' -timeout 30m -count=1 -v

# Run acceptance tests for a specific version (e.g., make testacc-version-v1_13_0)
testacc-version-%:
	@echo "Running tests for version: $*"
	@echo "Using API: $(BLASTSHIELD_HOST)"
	@echo ""
	BLASTSHIELD_HOST=$(BLASTSHIELD_HOST) BLASTSHIELD_TOKEN=$(BLASTSHIELD_TOKEN) \
		TF_ACC=1 go test ./internal/provider/$*/... -timeout 30m -count=1 -v

# Run acceptance tests for a specific resource in a specific version (e.g., make testacc-version-v1_13_0-node)
testacc-version-%-resource-%:
	@version=$$(echo "$*" | cut -d'-' -f1); \
	resource=$$(echo "$*" | cut -d'-' -f2); \
	BLASTSHIELD_HOST=$(BLASTSHIELD_HOST) BLASTSHIELD_TOKEN=$(BLASTSHIELD_TOKEN) \
		TF_ACC=1 go test -v ./internal/provider/$$version/... -run "TestAcc$$resource" -timeout 30m

# Fetch OpenAPI spec from remote server and save to openapi-specs/
fetch-openapi:
	@echo "Fetching OpenAPI spec from $(BLASTSHIELD_HOST)..."
	@mkdir -p openapi-specs
	@curl -s -H "Authorization: Bearer $(BLASTSHIELD_TOKEN)" \
		"$(BLASTSHIELD_HOST)/openapi.json" -o /tmp/openapi-temp.json
	@version=$$(python3 -c "import json; print(json.load(open('/tmp/openapi-temp.json'))['info']['version'])"); \
		output_file="openapi-specs/$$version.json"; \
		mv /tmp/openapi-temp.json "$$output_file"; \
		echo "Successfully downloaded OpenAPI spec version $$version to $$output_file"
	@echo "Run 'make generate' to generate code from the new spec"

# Generate code from all OpenAPI specs in openapi-specs/
generate:
	python3 -m venv .venv
	.venv/bin/pip install --quiet jinja2
	@for spec in openapi-specs/*.json; do \
		version=$$(python3 -c "import json; print(json.load(open('$$spec'))['info']['version'])"); \
		pkg_name=v$$(echo "$$version" | tr '.' '_'); \
		echo "Generating $$pkg_name from $$spec (API version $$version)"; \
		.venv/bin/python generate.py --spec "$$spec" \
			--output-dir "internal/provider/$$pkg_name" \
			--package "$$pkg_name"; \
	done
	.venv/bin/python generate_imports.py
	rm -rf .venv

fmt:
	go fmt ./...

lint:
	golangci-lint run

# Generate documentation from provider schemas
docs: build
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest generate --provider-name blastshield

clean:
	rm -f ${BINARY}
	rm -rf bin/
	rm -rf internal/provider/v*/
	rm -rf internal/provider/versionimports/

# Cleanup resources by name pattern (for old tests without tags)
# Requires curl and jq to be installed
cleanup-by-name-dryrun:
	@export BLASTSHIELD_HOST=$(BLASTSHIELD_HOST); \
	export BLASTSHIELD_TOKEN=$(BLASTSHIELD_TOKEN); \
	./scripts/cleanup-by-name.sh --dryrun

cleanup-by-name-debug:
	@export BLASTSHIELD_HOST=$(BLASTSHIELD_HOST); \
	export BLASTSHIELD_TOKEN=$(BLASTSHIELD_TOKEN); \
	./scripts/cleanup-by-name.sh --dryrun --debug

cleanup-by-name:
	@export BLASTSHIELD_HOST=$(BLASTSHIELD_HOST); \
	export BLASTSHIELD_TOKEN=$(BLASTSHIELD_TOKEN); \
	./scripts/cleanup-by-name.sh

# Show what test entities would be cleaned up (dry run)
# Requires curl and jq to be installed
cleanup-test-entities-dryrun:
	@echo "DRY RUN - Showing test entities that would be cleaned up"
	@echo "Using API: $(BLASTSHIELD_HOST)"
	@echo ""
	@echo "Nodes:"
	@curl -s -X GET "$(BLASTSHIELD_HOST)/nodes/?tags=blastshield_tf_testing_entity" \
		-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)" | \
		jq -r '.items[]? | "  [\(.id)] \(.name // "unnamed") (tags: \(.tags | to_entries | map("\(.key)=\(.value)") | join(", ")))"' || echo "  (none found or query failed)"
	@echo ""
	@echo "Endpoints:"
	@curl -s -X GET "$(BLASTSHIELD_HOST)/endpoints/?tags=blastshield_tf_testing_entity" \
		-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)" | \
		jq -r '.items[]? | "  [\(.id)] \(.name // "unnamed")"' || echo "  (none found or query failed)"
	@echo ""
	@echo "Groups:"
	@curl -s -X GET "$(BLASTSHIELD_HOST)/groups/?tags=blastshield_tf_testing_entity" \
		-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)" | \
		jq -r '.items[]? | "  [\(.id)] \(.name // "unnamed")"' || echo "  (none found or query failed)"
	@echo ""
	@echo "Services:"
	@curl -s -X GET "$(BLASTSHIELD_HOST)/services/?tags=blastshield_tf_testing_entity" \
		-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)" | \
		jq -r '.items[]? | "  [\(.id)] \(.name // "unnamed")"' || echo "  (none found or query failed)"
	@echo ""
	@echo "Policies:"
	@curl -s -X GET "$(BLASTSHIELD_HOST)/policies/?tags=blastshield_tf_testing_entity" \
		-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)" | \
		jq -r '.items[]? | "  [\(.id)] \(.name // "unnamed")"' || echo "  (none found or query failed)"
	@echo ""
	@echo "Egress Policies:"
	@curl -s -X GET "$(BLASTSHIELD_HOST)/egress_policies/?tags=blastshield_tf_testing_entity" \
		-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)" | \
		jq -r '.items[]? | "  [\(.id)] \(.name // "unnamed")"' || echo "  (none found or query failed)"
	@echo ""
	@echo "Proxies:"
	@curl -s -X GET "$(BLASTSHIELD_HOST)/proxies/?tags=blastshield_tf_testing_entity" \
		-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)" | \
		jq -r '.items[]? | "  [\(.id)] \(.name // "unnamed")"' || echo "  (none found or query failed)"
	@echo ""
	@echo "Event Log Rules:"
	@curl -s -X GET "$(BLASTSHIELD_HOST)/event_log_rules/?tags=blastshield_tf_testing_entity" \
		-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)" | \
		jq -r '.items[]? | "  [\(.id)] \(.name // "unnamed")"' || echo "  (none found or query failed)"
	@echo ""
	@echo "Run 'make cleanup-test-entities' to delete these entities"

# Cleanup test entities from the API (useful after test failures)
# Requires curl and jq to be installed
cleanup-test-entities:
	@echo "⚠️  WARNING: This will DELETE all test entities!"
	@echo "Run 'make cleanup-test-entities-dryrun' first to preview what will be deleted"
	@echo ""
	@read -p "Continue? [y/N] " -n 1 -r; \
	echo; \
	if [[ ! $$REPLY =~ ^[Yy]$$ ]]; then \
		echo "Cancelled."; \
		exit 1; \
	fi
	@echo ""
	@echo "Cleaning up test entities with tag 'blastshield_tf_testing_entity'..."
	@echo "Using API: $(BLASTSHIELD_HOST)"
	@echo ""
	@echo "Cleaning nodes..."
	@curl -s -X GET "$(BLASTSHIELD_HOST)/nodes/?tags=blastshield_tf_testing_entity" \
		-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)" | \
		jq -r '.items[]?.id // empty' | \
		while read id; do \
			echo "  Deleting node: $$id"; \
			curl -s -X DELETE "$(BLASTSHIELD_HOST)/nodes/$$id" \
				-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)"; \
		done
	@echo "Cleaning endpoints..."
	@curl -s -X GET "$(BLASTSHIELD_HOST)/endpoints/?tags=blastshield_tf_testing_entity" \
		-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)" | \
		jq -r '.items[]?.id // empty' | \
		while read id; do \
			echo "  Deleting endpoint: $$id"; \
			curl -s -X DELETE "$(BLASTSHIELD_HOST)/endpoints/$$id" \
				-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)"; \
		done
	@echo "Cleaning groups..."
	@curl -s -X GET "$(BLASTSHIELD_HOST)/groups/?tags=blastshield_tf_testing_entity" \
		-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)" | \
		jq -r '.items[]?.id // empty' | \
		while read id; do \
			echo "  Deleting group: $$id"; \
			curl -s -X DELETE "$(BLASTSHIELD_HOST)/groups/$$id" \
				-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)"; \
		done
	@echo "Cleaning services..."
	@curl -s -X GET "$(BLASTSHIELD_HOST)/services/?tags=blastshield_tf_testing_entity" \
		-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)" | \
		jq -r '.items[]?.id // empty' | \
		while read id; do \
			echo "  Deleting service: $$id"; \
			curl -s -X DELETE "$(BLASTSHIELD_HOST)/services/$$id" \
				-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)"; \
		done
	@echo "Cleaning policies..."
	@curl -s -X GET "$(BLASTSHIELD_HOST)/policies/?tags=blastshield_tf_testing_entity" \
		-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)" | \
		jq -r '.items[]?.id // empty' | \
		while read id; do \
			echo "  Deleting policy: $$id"; \
			curl -s -X DELETE "$(BLASTSHIELD_HOST)/policies/$$id" \
				-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)"; \
		done
	@echo "Cleaning egress policies..."
	@curl -s -X GET "$(BLASTSHIELD_HOST)/egress_policies/?tags=blastshield_tf_testing_entity" \
		-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)" | \
		jq -r '.items[]?.id // empty' | \
		while read id; do \
			echo "  Deleting egress policy: $$id"; \
			curl -s -X DELETE "$(BLASTSHIELD_HOST)/egress_policies/$$id" \
				-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)"; \
		done
	@echo "Cleaning proxies..."
	@curl -s -X GET "$(BLASTSHIELD_HOST)/proxies/?tags=blastshield_tf_testing_entity" \
		-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)" | \
		jq -r '.items[]?.id // empty' | \
		while read id; do \
			echo "  Deleting proxy: $$id"; \
			curl -s -X DELETE "$(BLASTSHIELD_HOST)/proxies/$$id" \
				-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)"; \
		done
	@echo "Cleaning event log rules..."
	@curl -s -X GET "$(BLASTSHIELD_HOST)/event_log_rules/?tags=blastshield_tf_testing_entity" \
		-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)" | \
		jq -r '.items[]?.id // empty' | \
		while read id; do \
			echo "  Deleting event log rule: $$id"; \
			curl -s -X DELETE "$(BLASTSHIELD_HOST)/event_log_rules/$$id" \
				-H "Authorization: Bearer $(BLASTSHIELD_TOKEN)"; \
		done
	@echo ""
	@echo "Cleanup complete!"

.PHONY: build release install test testacc fetch-openapi generate fmt lint docs clean cleanup-test-entities cleanup-test-entities-dryrun cleanup-by-name cleanup-by-name-dryrun cleanup-by-name-debug

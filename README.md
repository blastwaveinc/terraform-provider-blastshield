# Blastshield Terraform Provider

[![Terraform Registry](https://img.shields.io/badge/terraform-registry-blueviolet?logo=terraform)](https://registry.terraform.io/providers/blastwaveinc/blastshield/latest)
[![CI](https://github.com/blastwaveinc/terraform-provider-blastshield/actions/workflows/test.yml/badge.svg)](https://github.com/blastwaveinc/terraform-provider-blastshield/actions/workflows/test.yml)
[![Release](https://github.com/blastwaveinc/terraform-provider-blastshield/actions/workflows/release.yml/badge.svg)](https://github.com/blastwaveinc/terraform-provider-blastshield/actions/workflows/release.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/blastwaveinc/terraform-provider-blastshield)](https://github.com/blastwaveinc/terraform-provider-blastshield/blob/main/go.mod)
[![License](https://img.shields.io/github/license/blastwaveinc/terraform-provider-blastshield)](https://github.com/blastwaveinc/terraform-provider-blastshield/blob/main/LICENSE)

Terraform provider for managing [Blastshield](https://blastwave.com) resources.

- [Documentation](https://registry.terraform.io/providers/blastwaveinc/blastshield/latest/docs)
- [Terraform Registry](https://registry.terraform.io/providers/blastwaveinc/blastshield/latest)

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.21 (for building from source)
- [Python](https://python.org) >= 3.8 (for code generation)

## Development

The provider code is generated from OpenAPI specifications in `openapi-specs/` using Jinja2 templates. Generated code is not committed to the repository.

```bash
# Generate code from OpenAPI specs
make generate

# Build and install locally
make install

# Run unit tests
make test

# Run acceptance tests (requires a running Blastshield API)
make testacc

# Clean build artifacts and generated code
make clean
```

### Adding a New API Version

```bash
# Option 1: Fetch from a running Blastshield server
export BLASTSHIELD_HOST=https://your-server.com
export BLASTSHIELD_TOKEN=your-token
make fetch-openapi

# Option 2: Place the spec manually
cp /path/to/spec.json openapi-specs/1.14.0.json

# Then regenerate and build
make generate
go build ./...
```

## License

Apache 2.0 - See [LICENSE](LICENSE) for details.

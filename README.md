# Blastshield Terraform Provider

Terraform provider for managing [Blastshield](https://blastwave.com) resources.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.21 (for building from source)
- [Python](https://python.org) >= 3.8 (for code generation)

## Installation

### From Source

```bash
git clone https://github.com/blastwaveinc/terraform-provider-blastshield.git
cd terraform-provider-blastshield

# Replace openapi.json with the version matching your BlastShield environment
# You can export it from your Blastshield orchestrator at https://<orchestrator_hostname>:8000/openapi.json
cp /path/to/your/openapi.json .

# Build and install
make install
```

This will generate the provider code from the OpenAPI spec, build the binary, and install it to your local Terraform plugins directory.

## Configuration

Configure the provider with your Blastshield API credentials:

```hcl
provider "blastshield" {
  host  = "https://orchestrator.blastshield.io:8000"
  token = "your-api-token"
}
```

### Environment Variables

You can also configure the provider using environment variables:

```bash
export BLASTSHIELD_HOST="https://orchestrator.blastshield.io:8000"
export BLASTSHIELD_TOKEN="your-api-token"
```

## Usage

### Managing Nodes

```hcl
resource "blastshield_node" "example" {
  name       = "my-node"
  node_type  = "A"
  api_access = false
  tags = {
    environment = "production"
  }
}

# The invitation field contains base64-encoded registration data
output "node_invitation" {
  value     = blastshield_node.example.invitation
  sensitive = true
}
```

### Managing Groups

```hcl
resource "blastshield_group" "developers" {
  name      = "developers"
  users     = []
  endpoints = []
  tags = {
    team = "engineering"
  }
}
```

### Managing Endpoints

```hcl
resource "blastshield_endpoint" "web_server" {
  name    = "web-server"
  node_id = blastshield_node.example.id
  enabled = true
  address = "10.0.0.1"

  groups = [
    {
      id      = blastshield_group.developers.id
      expires = 0
    }
  ]
}
```

### Managing Policies

```hcl
resource "blastshield_service" "https" {
  name = "https"
  protocols = [
    {
      ip_protocol = 6
      ports       = ["443"]
    }
  ]
  tags = {
    port = "443"
  }
}

resource "blastshield_policy" "allow_web" {
  name        = "allow-web-access"
  enabled     = true
  log         = true
  from_groups = [blastshield_group.developers.id]
  to_groups   = [blastshield_group.developers.id]
  services    = [blastshield_service.https.id]
}
```

### Managing Egress Policies

```hcl
resource "blastshield_egresspolicy" "allow_external" {
  name                  = "allow-external"
  enabled               = true
  allow_all_dns_queries = false
  groups                = [blastshield_group.developers.id]
  services              = []
  destinations          = ["example.com", "*.github.com"]
  dns_names             = []
}
```

### Managing Proxies

```hcl
resource "blastshield_proxy" "web_proxy" {
  name        = "web-proxy"
  proxy_port  = 8080
  domains     = ["internal.example.com"]
  groups      = [blastshield_group.developers.id]
  exit_agents = [blastshield_node.example.id]
}
```

### Data Sources

Query existing resources:

```hcl
# Get a single node by ID
data "blastshield_node" "existing" {
  id = "node-id-here"
}

# List all nodes
data "blastshield_nodes" "all" {}

# List nodes with filters
data "blastshield_nodes" "filtered" {
  name = "production-*"
}
```

## Resources

| Resource | Description |
|----------|-------------|
| `blastshield_node` | Manages Blastshield nodes (agents) |
| `blastshield_endpoint` | Manages endpoints on nodes |
| `blastshield_group` | Manages groups for access control |
| `blastshield_service` | Manages service definitions |
| `blastshield_policy` | Manages network policies between groups |
| `blastshield_egresspolicy` | Manages egress policies for external access |
| `blastshield_proxy` | Manages proxy configurations |
| `blastshield_eventlogrule` | Manages event logging rules |

## Data Sources

Each resource has corresponding data sources for reading existing resources:

- `blastshield_node` / `blastshield_nodes`
- `blastshield_endpoint` / `blastshield_endpoints`
- `blastshield_group` / `blastshield_groups`
- `blastshield_service` / `blastshield_services`
- `blastshield_policy` / `blastshield_policies`
- `blastshield_egresspolicy` / `blastshield_egresspolicies`
- `blastshield_proxy` / `blastshield_proxies`
- `blastshield_eventlogrule` / `blastshield_eventlogrules`

## Development

### Code Generation

The provider code is generated from an OpenAPI specification using Jinja2 templates. The generated code is not committed to the repository.

```bash
# Generate code from OpenAPI spec (creates a temporary Python venv)
make generate

# Build the provider
make build

# Run acceptance tests (requires a running Blastshield API)
make testacc

# Clean build artifacts and generated code
make clean
```

### Project Structure

```
.
├── generate.py              # Code generator script
├── openapi.json             # OpenAPI spec (replace with your version)
├── codegen-templates/       # Jinja2 templates for Go code generation
│   ├── client.go.j2
│   ├── data_source.go.j2
│   ├── helpers.go.j2
│   ├── macros.j2
│   ├── provider.go.j2
│   ├── resource.go.j2
│   ├── schemas.go.j2
│   └── types.go.j2
├── internal/provider/
│   ├── client.go            # HTTP client implementation
│   ├── generated/           # Generated code (not in git)
│   └── *_test.go            # Acceptance tests
└── examples/                # Example Terraform configurations
```

### Updating the OpenAPI Spec

When your Blastshield environment is updated, regenerate the provider:

1. Export the new OpenAPI spec from your orchestrator
2. Replace `openapi.json` with the new version
3. Run `make clean && make build`
4. Run `make testacc` to verify the changes

## License

Apache 2.0 - See [LICENSE](LICENSE) for details.

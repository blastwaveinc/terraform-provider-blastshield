# List all endpoints
data "blastshield_endpoints" "all" {}

# Filter endpoints by name
data "blastshield_endpoints" "web" {
  name = ["web-server-01"]
}

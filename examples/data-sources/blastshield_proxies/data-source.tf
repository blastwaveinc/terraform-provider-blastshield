# List all proxies
data "blastshield_proxies" "all" {}

# Filter proxies by name
data "blastshield_proxies" "web" {
  name = ["web-proxy"]
}

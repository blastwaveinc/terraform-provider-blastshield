# List all services
data "blastshield_services" "all" {}

# Filter services by name
data "blastshield_services" "https" {
  name = ["HTTPS"]
}

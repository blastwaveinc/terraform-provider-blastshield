# List all groups
data "blastshield_groups" "all" {}

# Filter groups by name
data "blastshield_groups" "web" {
  name = ["web-servers"]
}

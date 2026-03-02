# List all policies
data "blastshield_policies" "all" {}

# Filter policies by name
data "blastshield_policies" "web" {
  name = ["allow-web-traffic"]
}

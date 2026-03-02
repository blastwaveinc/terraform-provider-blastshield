# List all egress policies
data "blastshield_egress_policies" "all" {}

# Filter egress policies by name
data "blastshield_egress_policies" "updates" {
  name = ["allow-package-updates"]
}

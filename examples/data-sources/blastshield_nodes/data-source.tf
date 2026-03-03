# List all nodes
data "blastshield_nodes" "all" {}

# Filter nodes by type
data "blastshield_nodes" "gateways" {
  node_type = ["gateway"]
}

resource "blastshield_node" "gateway" {
  name          = "us-east-gateway"
  node_type     = "G" # Gateway
  endpoint_mode = "N" # NAT addressing mode (required for gateways)

  tags = {
    environment = "production"
    region      = "us-east-1"
  }
}

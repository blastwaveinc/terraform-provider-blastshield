resource "blastshield_endpoint" "web_server" {
  name     = "web-server-01"
  node_id  = blastshield_node.gateway.id
  enabled  = true
  endpoint = "10.0.1.100"

  tags = {
    environment = "production"
    role        = "web"
  }
}

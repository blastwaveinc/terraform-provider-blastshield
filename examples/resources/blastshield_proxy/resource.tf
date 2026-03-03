resource "blastshield_proxy" "web_proxy" {
  name    = "web-proxy"
  domains = ["app.example.com"]
  groups  = [blastshield_group.web_servers.id]

  tags = {
    environment = "production"
  }
}

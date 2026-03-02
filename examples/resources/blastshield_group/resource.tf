resource "blastshield_group" "web_servers" {
  name = "web-servers"

  endpoints = [
    {
      id      = blastshield_endpoint.web_server.id
      expires = 0
    }
  ]

  users = []

  tags = {
    environment = "production"
  }
}

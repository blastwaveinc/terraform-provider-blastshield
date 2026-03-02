resource "blastshield_service" "https" {
  name = "HTTPS"

  protocols = [
    {
      ip_protocol = 6
      ports       = ["443"]
    }
  ]

  tags = {
    type = "web"
  }
}

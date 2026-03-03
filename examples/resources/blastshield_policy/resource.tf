resource "blastshield_policy" "allow_web" {
  name        = "allow-web-traffic"
  enabled     = true
  log         = true
  from_groups = [blastshield_group.web_servers.id]
  to_groups   = [blastshield_group.web_servers.id]
  services    = [blastshield_service.https.id]

  tags = {
    environment = "production"
  }
}

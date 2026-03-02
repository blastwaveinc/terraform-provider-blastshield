resource "blastshield_egress_policy" "allow_updates" {
  name                  = "allow-package-updates"
  enabled               = true
  allow_all_dns_queries = false
  services              = [blastshield_service.https.id]
  groups                = [blastshield_group.web_servers.id]
  destinations          = ["0.0.0.0/0"]

  dns_names = [
    {
      name      = "packages.example.com"
      recursive = false
    }
  ]

  tags = {
    environment = "production"
  }
}

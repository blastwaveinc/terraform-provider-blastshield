provider "blastshield" {
  host  = "https://blastshield.example.com"
  token = var.blastshield_token

  # Optional: tags applied to every resource that supports tags.
  # Resource-level tags take precedence on key conflicts.
  default_tags = {
    managed_by = "terraform"
  }
}

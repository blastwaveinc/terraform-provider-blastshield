resource "blastshield_event_log_rule" "failed_auth" {
  name    = "alert-on-failed-auth"
  enabled = true

  conditions = [
    {
      condition_type = "category"
      operator       = "eq"
      value          = "auth"
    }
  ]

  actions          = ["email-notification"]
  email_recipients = ["security@example.com"]
  apply_to_groups  = [blastshield_group.web_servers.id]

  tags = {
    team = "security"
  }
}

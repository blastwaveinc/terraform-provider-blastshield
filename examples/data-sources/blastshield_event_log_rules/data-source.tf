# List all event log rules
data "blastshield_event_log_rules" "all" {}

# Filter event log rules by name
data "blastshield_event_log_rules" "auth_alerts" {
  name = ["alert-on-failed-auth"]
}

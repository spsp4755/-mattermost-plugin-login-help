# Offline Deployment Checklist

Use this checklist before importing the plugin into a restricted Mattermost environment.

## Prerequisites

- Mattermost server version 9.0.0 or later
- JSON audit logging enabled
- SMTP configured to an internal mail relay
- Internal Confluence or intranet guide URL prepared
- Plugin upload enabled for administrators

## Recommended Plugin Configuration

- `Enabled`: `true`
- `AuditLogPath`: absolute path to the JSON audit log file on the Mattermost host
- `FailureThreshold`: `3`
- `WindowMinutes`: `15`
- `CooldownMinutes`: `60`
- `ConfluenceURL`: internal recovery guide URL
- `PollIntervalSeconds`: `5`
- `StartFromEnd`: `true`
- `ResetOnSuccess`: `true`
- `OnlyLocalAccounts`: `true`

## Operational Notes

- This plugin sends a recovery guide email. It does not reset passwords by itself.
- In a multi-node Mattermost deployment, use a shared audit log feed or deploy a node-specific watcher strategy before enabling the plugin broadly.
- If your organization uses LDAP, SAML, or OIDC, keep `OnlyLocalAccounts=true` unless the linked guide covers those identity systems.
- Review mail relay rules so the notification cannot be abused as an email flood path. The built-in cooldown helps, but mail policies should still exist upstream.
- Test the plugin with the admin test-mail endpoint before enabling the watcher in production.

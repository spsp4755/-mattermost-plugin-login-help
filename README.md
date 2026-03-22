# Mattermost Login Help Mailer

This Mattermost server plugin watches the JSON audit log for repeated login failures and sends a help email that points the user to an internal Confluence password reset guide.

It is designed for restricted or air-gapped environments where:

- Mattermost can write JSON audit logs locally.
- Mattermost SMTP is already configured to reach an internal mail relay.
- The recovery guide lives on an internal Confluence page or other intranet URL.

The plugin does not call external services, generate password reset tokens, or change passwords automatically.

## What It Does

1. Reads new entries from a configured Mattermost JSON audit log file.
2. Counts failed `login` events per user inside a rolling time window.
3. Sends one help email when the configured threshold is reached.
4. Applies a cooldown so the same user is not spammed repeatedly.
5. Clears the stored failure counter after a successful login when that option is enabled.

## Recommended Settings For Air-Gapped Use

- Enable Mattermost audit logging in JSON format.
- Use an absolute `AuditLogPath` that the plugin process can read.
- Keep `StartFromEnd=true` on first deployment to avoid replaying historical failures.
- Keep `OnlyLocalAccounts=true` unless your Confluence guide also applies to LDAP, SAML, or OIDC users.
- Use an internal `http` or `https` Confluence URL only.
- Configure SMTP in Mattermost to point to your internal mail relay before enabling the plugin.

## Admin API

- `GET /plugins/com.mattermost.login-help-mailer/api/v1/status`
- `POST /plugins/com.mattermost.login-help-mailer/api/v1/test-mail`

Both endpoints require a logged-in Mattermost system administrator session.

## Build

On Unix-like systems:

```bash
make bundle
```

On Windows PowerShell:

```powershell
.\build.ps1
```

The bundle is written to:

```text
dist/com.mattermost.login-help-mailer-0.1.1.tar.gz
```

## Deployment Notes

See `docs/OFFLINE-DEPLOYMENT.md` for an example rollout checklist for a restricted network.

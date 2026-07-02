# JooMail

## Backend

Run the Go webmail API server:

```sh
go run ./cmd/joomaild
```

The server listens on `127.0.0.1:8080` by default. Set `JOOMAIL_ADDR` to override it.
Set `JOOMAIL_STATIC_DIR` to serve the built frontend from the same process.

The product API is backed by live IMAP/SMTP data. Configure the mail server and session settings with environment variables such as:

- `JOOMAIL_IMAP_HOST`, `JOOMAIL_IMAP_PORT`, `JOOMAIL_IMAP_TLS`, `JOOMAIL_IMAP_USER_FORMAT`
- `JOOMAIL_SMTP_HOST`, `JOOMAIL_SMTP_PORT`, `JOOMAIL_SMTP_TLS`, `JOOMAIL_SMTP_STARTTLS`, `JOOMAIL_SMTP_USER_FORMAT`
- `JOOMAIL_LOGIN_DOMAIN`
- `JOOMAIL_SESSION_SECRET`, `JOOMAIL_CREDENTIAL_KEY`, `JOOMAIL_CREDENTIAL_DIR`

Initial API endpoints:

- `GET /api/health`
- `POST /api/login`
- `POST /api/logout`
- `GET /api/accounts`
- `GET /api/accounts/{accountID}/mailboxes/{mailboxID}/messages` (`q` search, optional `scope=mailbox|account`)
- `GET /api/messages/{messageID}`
- `GET /api/messages/{messageID}/attachments/{attachmentID}`
- `PATCH /api/messages/{messageID}/flagged`
- `PATCH /api/messages/{messageID}/seen`
- `POST /api/messages/{messageID}/move`
- `POST /api/send`

## Deploy

Deployment uses the self-hosted GitHub Actions runner and follows the PillowCare
server pattern.

Deploy manually from GitHub Actions, or push a release tag:

```sh
git tag joomail-v0.1.0
git push origin joomail-v0.1.0
```

The workflow builds the Vite frontend, builds the Go backend, installs artifacts
under `/opt/JooMail`, and restarts the `joomail` systemd service on the Ubuntu
server.

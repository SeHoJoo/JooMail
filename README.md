# JooMail

## Backend

Run the Go webmail API server:

```sh
go run ./cmd/joomaild
```

The server listens on `127.0.0.1:8080` by default. Set `JOOMAIL_ADDR` to override it.
Set `JOOMAIL_STATIC_DIR` to serve the built frontend from the same process.

The product API is backed by live IMAP/SMTP data. Configure the mail server and session settings with environment variables such as:

- `JOOMAIL_IMAP_HOST`, `JOOMAIL_IMAP_PORT`: required IMAP endpoint.
- `JOOMAIL_IMAP_TLS`: optional implicit TLS toggle for IMAP connections.
- `JOOMAIL_IMAP_USER_FORMAT`: optional login username mapping. The current backend supports the configured server pattern; do not add account systems outside IMAP LOGIN in this phase.
- `JOOMAIL_SMTP_HOST`, `JOOMAIL_SMTP_PORT`: required SMTP endpoint for send.
- `JOOMAIL_SMTP_TLS`: optional implicit TLS toggle for SMTP. Port `465` also uses implicit TLS.
- `JOOMAIL_SMTP_STARTTLS`: optional STARTTLS upgrade for non-implicit SMTP connections.
- `JOOMAIL_SMTP_USER_FORMAT`: optional SMTP username mapping. Current send support expects the approved localpart mode.
- `JOOMAIL_MANAGESIEVE_HOST`, `JOOMAIL_MANAGESIEVE_PORT`: optional ManageSieve endpoint for server-side rules.
- `JOOMAIL_MANAGESIEVE_TLS`: optional implicit TLS toggle for ManageSieve connections.
- `JOOMAIL_LOGIN_DOMAIN`: optional domain appended during login flows when applicable.
- `JOOMAIL_SESSION_SECRET`: required HMAC signing secret for session cookies.
- `JOOMAIL_CREDENTIAL_KEY`: required local encryption key for stored session credentials.
- `JOOMAIL_CREDENTIAL_DIR`: required local directory for encrypted per-session credential files.

Credential files are created after successful IMAP login so the API can open
live IMAP/SMTP sessions for that browser session. Logout deletes the stored
credential and expires the session cookie. Do not commit credential files,
keys, secrets, or environment values.

Initial API endpoints:

- `GET /api/health` returns `{"status":"ok"}` for smoke checks.
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
- `POST /api/drafts`
- `GET /api/accounts/{accountID}/rules`
- `PUT /api/accounts/{accountID}/rules`

`POST /api/send` trims To/Cc/Bcc recipients and rejects missing or malformed
addresses with `400` before opening an SMTP connection. Bcc recipients are used
for SMTP delivery but are not written to outgoing message headers.
Send requests are capped at 32 MiB. SMTP auth, recipient, DATA, and Sent append
failures return the generic `502 failed to send message` response. A Sent append
failure currently fails the API response after SMTP delivery instead of adding a
new warning field to the response contract.

`POST /api/drafts` accepts the same JSON or multipart shape as `POST /api/send`,
but permits incomplete recipients or subject. It appends the generated message to
the account's Drafts mailbox with the IMAP `\Draft` flag and returns
`{"status":"saved"}`.

Message list responses currently return the newest 50 live IMAP matches. Future
load-more support should extend this route with optional query parameters such
as `limit` plus a UID/date cursor, while keeping the existing `messages` JSON
field stable for current clients. Account-scope search remains limited to the
current account, searches each selectable mailbox live through IMAP, caps each
mailbox fetch, and caps the merged result after sorting.

Non-ASCII search terms are sent with `CHARSET UTF-8` first. If the IMAP server
rejects that charset search, JooMail retries the same `TEXT` search without the
charset prefix before returning a search failure.

Message move uses IMAP `UID MOVE` when the server supports it. If `MOVE` is not
accepted, JooMail falls back to `UID COPY`, `UID STORE +FLAGS.SILENT
(\Deleted)`, and `EXPUNGE` on the selected source mailbox.

Rules are managed through ManageSieve when `JOOMAIL_MANAGESIEVE_HOST` and
`JOOMAIL_MANAGESIEVE_PORT` are configured. If ManageSieve is not configured, the
rules API returns `503 rules are unavailable` and does not touch Sieve files or
mail-server configuration. Rules authenticate with the current session's stored
mail credential and write only a delimited `BEGIN JOOMAIL RULES` / `END JOOMAIL
RULES` block in a JooMail-managed Sieve script. JooMail currently supports
sender email/domain `contains` or `equals`, subject `contains`, and safe folder
moves including Spam and Trash. Labels and destructive discard/block rules are
not implemented in this phase.

## Deploy

Deployment uses the self-hosted GitHub Actions runner and follows the PillowCare
server pattern.

Deploy manually from GitHub Actions, or push a release tag:

```sh
git tag joomail-v0.1.14
git push origin joomail-v0.1.14
```

The workflow builds the Vite frontend, builds the Go backend, installs artifacts
under `/opt/JooMail`, and restarts the `joomail` systemd service on the Ubuntu
server.

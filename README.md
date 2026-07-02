# JooMail

## Backend

Run the mock Go API server:

```sh
go run ./cmd/joomaild
```

The server listens on `127.0.0.1:8080` by default. Set `JOOMAIL_ADDR` to override it.
Set `JOOMAIL_STATIC_DIR` to serve the built frontend from the same process.

Initial mock endpoints:

- `GET /api/health`
- `GET /api/accounts`
- `GET /api/accounts/{accountID}/mailboxes/{mailboxID}/messages`
- `GET /api/messages/{messageID}`

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

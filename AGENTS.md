# JooMail Agent Instructions

## Project Goal
JooMail is a browser-based webmail client for a server that will later use Dovecot/Postfix.

## Phase Scope
Current phase: backend foundation.

The frontend UI prototype is already in place. For this phase:
- Add a Go backend foundation with a small HTTP API surface.
- Use mock account/mail data until real IMAP/SMTP integration is explicitly started.
- Keep API responses shaped around already-parsed mail content; frontend must not parse raw MIME.
- Use the Go standard library unless a dependency is clearly justified and approved.

Future phases may add:
- IMAP/SMTP integration
- Authentication
- Persistence
- Dovecot/Postfix deployment/configuration support

## Deployment
- GitHub Actions deploys through the JooMail self-hosted runner.
- Push a `joomail-v*` tag or run `.github/workflows/deploy.yml` manually to deploy.
- Deployment follows the PillowCare server pattern: build on the runner, upload artifacts to `mail.good-night.co.kr`, install under `/opt/JooMail`, and restart the `joomail` systemd service.
- The deployed service uses `JOOMAIL_ADDR=127.0.0.1:8081` and serves built frontend files from `/opt/JooMail/www`.
- Keep reverse proxy, TLS, DNS, and firewall changes outside the repo unless explicitly requested.

## Required Reading Before Implementation
Before editing code, read:
- `docs/webmail-ui-plan.md`

Use the docs for product decisions and feature scope. Use the Figma-derived UI only for frontend visual work.

## Do Not Touch In Current Phase
- Dovecot/Postfix configuration
- Real IMAP/SMTP integration
- Authentication/session systems
- Database, migrations, persistence
- Docker, deployment, CI
- Secrets, credentials, environment files

## Engineering Rules
- Make the smallest change that completes the requested backend step.
- Do not add features beyond `docs/webmail-ui-plan.md`.
- Ask before adding dependencies beyond the Go standard library or the existing Vite/React/TypeScript/Tailwind setup.
- Keep backend code readable and scoped: `cmd/joomaild`, `internal/httpapi`, and mock data only until real integration begins.
- Verify backend changes with `go test ./...` before reporting completion.

## Frontend Rules
- Keep components readable and scoped: `AppShell`, `Sidebar`, `MessageList`, `MessageRow`, `ReadingPane`, `Toolbar`, `ComposePanel`, and state views.
- Verify with build, typecheck, or lint when available before reporting completion.

## Design Rules
- Match the Figma design closely.
- Prioritize a dense, calm, operational webmail UI.
- Do not create a landing page.
- Do not use decorative gradients, oversized hero sections, or marketing-style layout.
- Desktop must support a three-pane inbox layout.
- Mobile must match the mobile inbox frame and avoid overlapping controls.

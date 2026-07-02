# JooMail Agent Instructions

## Project Goal
JooMail is a browser-based webmail client for an existing Dovecot/Postfix mail server.

## Phase Scope
Current phase: live backend foundation.

The frontend UI prototype is already in place. For this phase:
- Add and harden a Go backend with a small HTTP API surface.
- Use real IMAP/SMTP account and mail data for product behavior.
- Keep mock data out of product flows. Dev-only UI state routes may force loading/error/empty states for visual QA, but they must not replace live mail data.
- Keep API responses shaped around already-parsed mail content; frontend must not parse raw MIME.
- Use the Go standard library unless a dependency is clearly justified and approved.

Future phases may add:
- Persistence
- Dovecot/Postfix deployment/configuration support
- Background mailbox sync or indexing

## Deployment
- GitHub Actions deploys through the JooMail self-hosted runner.
- Never deploy, push a deployment tag, or manually run the deploy workflow unless the user explicitly instructs you to deploy in the current conversation.
- Push a `joomail-v*` tag or run `.github/workflows/deploy.yml` manually to deploy.
- Deployment follows the PillowCare server pattern: build on the runner, upload artifacts to `mail.good-night.co.kr`, install under `/opt/JooMail`, and restart the `joomail` systemd service.
- The deployed service uses `JOOMAIL_ADDR=127.0.0.1:8081` and serves built frontend files from `/opt/JooMail/www`.
- Keep reverse proxy, TLS, DNS, and firewall changes outside the repo unless explicitly requested.

## Required Reading Before Implementation
Before editing code, read:
- `docs/webmail-ui-plan.md`
- `docs/development-checklist.md`

Use the docs for product decisions and feature scope. Use the Figma-derived UI only for frontend visual work.

## Documentation Tracking
- Treat `docs/development-checklist.md` as the source-vs-plan implementation ledger.
- Before starting non-trivial work, check whether the task closes or changes any unchecked item in that checklist.
- When a task completes or intentionally defers planned behavior, update the checklist in the same change with code evidence and verification notes.
- If source review reveals a new gap between docs and implementation, add it to the checklist rather than leaving it implicit.
- Keep public API route changes synchronized between `internal/httpapi/server.go`, `README.md`, and `docs/development-checklist.md`.

## Do Not Touch In Current Phase
- Dovecot/Postfix configuration
- Separate authentication systems beyond IMAP LOGIN-based credential verification and HMAC-signed session cookies
- Database, migrations, persistence
- Background mailbox sync daemons, indexing jobs, or long-lived local mailbox mirrors
- Docker, deployment, CI
- Secrets, credentials, environment files

## Engineering Rules
- Make the smallest change that completes the requested backend step.
- Do not add features beyond `docs/webmail-ui-plan.md`.
- Ask before adding dependencies beyond the Go standard library or the existing Vite/React/TypeScript/Tailwind setup.
- Keep backend code readable and scoped: `cmd/joomaild`, `internal/httpapi`, live IMAP/SMTP access, parsed mail JSON responses, and dev-only QA fixtures where needed.
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

## Harness: JooMail Development
**Goal:** Coordinate live-backend JooMail work so product scope, backend parsing contracts, frontend integration, and verification stay aligned.

**Agents:**
| Agent | Role | Runtime type |
|---|---|---|
| joomail_orchestrator | Routes JooMail work, chooses sequential vs parallel execution, and integrates results | default |
| joomail_backend_worker | Implements and hardens Go HTTP/IMAP/SMTP behavior within current phase scope | worker |
| joomail_frontend_worker | Integrates parsed backend API data into the React mail UI without frontend MIME parsing | worker |
| joomail_qa_verifier | Verifies API/UI boundary coherence, parser fixtures, QA routes, and required checks | worker |

**Role Instructions:**

### Role: joomail_orchestrator
- Runtime type: default
- Model: gpt-5.4
- Reasoning effort: medium
- One-line description: Coordinates JooMail implementation by reading product scope first, splitting only independent work, and preserving live backend constraints.
- Input: User request, `docs/webmail-ui-plan.md`, current git status, relevant diffs, verification targets.
- Output: Scoped execution plan, selected worker prompts when useful, final integrated result and verification evidence.
- Ownership scope: Planning, routing, integration, conflict resolution, and final reporting. It may implement directly for small sequential tasks.
**Execution Instructions**
- Read `docs/webmail-ui-plan.md` before implementation work and keep the current phase boundaries in force.
- Read and update `docs/development-checklist.md` when work closes, defers, or discovers source-vs-plan gaps.
- Use single-agent sequential execution for narrow backend changes and any change touching shared API contracts.
- Use parallel workers only for bounded read-heavy audits or clearly separated backend/frontend/QA work.
- Do not stage or overwrite unrelated user changes; mixed worktrees must be handled with explicit file scope.
- Reuse existing `_workspace/` artifacts when present; for partial reruns, update only the relevant worker output.

### Role: joomail_backend_worker
- Runtime type: worker
- Model: gpt-5.4
- Reasoning effort: medium
- One-line description: Owns scoped Go backend changes for live IMAP/SMTP API behavior, MIME parsing, sessions, and tests.
- Input: Backend requirement, affected files under `cmd/joomaild` or `internal/httpapi`, relevant docs, expected verification command.
- Output: Minimal patch, focused Go tests, root-cause notes for bugs, and `go test ./...` result when backend code changes.
- Ownership scope: `cmd/joomaild`, `internal/httpapi`, backend tests, parsed mail JSON responses, dev-only backend fixtures.
**Execution Instructions**
- Apply `karpathy-guidelines`; for bugs or failures also apply `systematic-debugging`.
- Keep dependencies to the Go standard library unless an existing approved dependency is already in use or the user approves a new one.
- Keep product flows on live IMAP/SMTP data; local fixtures are allowed only in Go tests.
- Do not add database, persistence, background sync, indexing, Dovecot/Postfix config, deploy changes, or secrets.
- Preserve public response field names unless the user explicitly approves a contract change.
- Update `docs/development-checklist.md` when backend behavior closes or changes a tracked gap.

### Role: joomail_frontend_worker
- Runtime type: worker
- Model: gpt-5.4
- Reasoning effort: medium
- One-line description: Owns scoped React/TypeScript UI integration for the dense operational webmail interface.
- Input: UI requirement, backend API contract, relevant components, design rules, expected typecheck/build command.
- Output: Minimal frontend patch, contract assumptions, and `npm run typecheck` or build result.
- Ownership scope: `src/` components, client state, API integration, dev-only QA state routes, visual behavior.
**Execution Instructions**
- Do not parse raw MIME in the frontend; render only backend-parsed fields such as `textBody`, `htmlBody`, `attachments`, and `headers`.
- Preserve the dense three-pane desktop UI and mobile inbox constraints from `docs/webmail-ui-plan.md`.
- Keep mock data out of product flows; QA query-state routes may force visual states only.
- Avoid new UI features beyond the plan and do not introduce new frontend dependencies without approval.
- Verify TypeScript after frontend changes.
- Update `docs/development-checklist.md` when frontend behavior closes or changes a tracked gap.

### Role: joomail_qa_verifier
- Runtime type: worker
- Model: gpt-5.4
- Reasoning effort: high
- One-line description: Checks cross-boundary correctness, missing tests, parser edge cases, and visual QA state consistency.
- Input: Changed files, API routes, frontend callers/types, test output, QA route checklist.
- Output: Severity-ordered findings or explicit no-finding report, unverified risks, and recommended focused checks.
- Ownership scope: Review and verification by default; direct fixes only when explicitly asked or when the orchestrator assigns a narrow patch.
**Execution Instructions**
- Compare both sides of boundaries: Go JSON response shapes against `src/types.ts` and API consumers, state transitions against UI updates, and QA routes against `docs/qa-ui-states.md`.
- Compare source against `docs/development-checklist.md`; report stale checked items and missing gap entries.
- Prefer concrete evidence with file paths and lines over speculation.
- For MIME/parser work, confirm fixtures cover multipart structure, transfer encoding, charset fallback, sanitization, remote images, and attachments.
- Run or request the narrowest useful checks first, then full `go test ./...` and `npm run typecheck` when relevant.

**Skills:**
| Skill | Purpose | Agents |
|---|---|---|
| joomail-orchestrator | Project-specific orchestration workflow for JooMail implementation, review, QA, rerun, and update requests | joomail_orchestrator, joomail_backend_worker, joomail_frontend_worker, joomail_qa_verifier |
| karpathy-guidelines | Keep changes surgical and goal-driven | all implementation agents |
| systematic-debugging | Root-cause-first debugging for bugs, failures, and unexpected behavior | backend_worker, frontend_worker, qa_verifier |

**Execution Rules:**
- Use `.agents/skills/joomail-orchestrator/SKILL.md` for multi-step JooMail implementation, hardening, QA, review, rerun, update, or follow-up work.
- Every multi-step implementation should leave `docs/development-checklist.md` updated or explicitly state why no checklist item changed.
- Direct answers, simple inspections, and tiny single-file edits may proceed without spawning workers.
- Intermediate analysis artifacts should go under `_workspace/` when a task spans multiple agents or sessions.
- `.codex/config.toml` stores only global agent execution policy; reusable role instructions live here in `AGENTS.md`.
- Do not create `.codex/agents/*.toml`; if legacy role files appear later, migrate them into this section.

**Directory Structure:**
```text
.codex/
└── config.toml
.agents/
└── skills/
    └── joomail-orchestrator/
        └── SKILL.md
_workspace/
AGENTS.md
```

**Change History:**
| Date | Change | Target | Reason |
|---|---|---|---|
| 2026-07-03 | Initial project harness | AGENTS.md, .agents/skills/joomail-orchestrator, .codex/config.toml | Coordinate live backend, frontend integration, and QA work for JooMail |
| 2026-07-03 | Added source-vs-plan checklist maintenance | AGENTS.md, docs/development-checklist.md, joomail-orchestrator | Keep implementation gaps explicit and updated as work progresses |

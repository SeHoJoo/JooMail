# JooMail Development Checklist

This checklist tracks the gap between `docs/webmail-ui-plan.md` and the current source tree.

Last audited: 2026-07-03  
Audit basis: `AGENTS.md`, `README.md`, `docs/webmail-ui-plan.md`, `docs/qa-ui-states.md`, `internal/httpapi/*`, `src/*`

## Maintenance Rules

- Update this file whenever a task implements, removes, or materially changes planned behavior.
- Before starting a non-trivial task, check whether it closes or changes any unchecked item here.
- When completing an item, change `[ ]` to `[x]`, add the implementation evidence, and note the verification command.
- If source review reveals a new gap between docs and implementation, add it here in the same change.
- Do not mark an item complete based only on intent. Use code evidence and a passing verification command.

## Current Implemented Baseline

- [x] Live IMAP login-backed session flow exists.
  Evidence: `POST /api/login`, `GET /api/accounts`, HMAC session cookie, encrypted credential store.
  Verification: existing `internal/httpapi/server_test.go` auth/session coverage.
- [x] Live mailbox and message APIs exist.
  Evidence: accounts, mailbox tree, message summaries, message detail, attachment download, flag, seen, move routes.
  Verification: existing `internal/httpapi/server_test.go` route coverage.
- [x] MIME parsing is backend-owned and covered by parser fixtures.
  Evidence: multipart alternative/mixed/related, transfer encoding, charset, sanitize, remote image, attachment tests.
  Verification: `go test ./...`.
- [x] SMTP send with attachments and Sent append exists.
  Evidence: `POST /api/send`, `internal/httpapi/smtp.go`, SMTP and append tests.
  Verification: existing send route tests.
- [x] React UI consumes backend API for product flow after login.
  Evidence: `src/App.tsx` loads `/api/accounts`, message summaries, details, send, move, seen, flagged.
  Verification: `npm run typecheck`.
- [x] Dev-only QA routes exist for visual states.
  Evidence: `/?qa=loading`, `/?qa=error`, `/?qa=empty`, `/?qa=search`, `/?qa=search-empty`, `/?qa=multiselect`, `/?qa=compose`.
  Verification: `docs/qa-ui-states.md` route list and `src/App.tsx` QA state handling.

## Backend Gaps

- [x] Replace list-then-filter search with IMAP server-side search.
  Evidence: `messageSummaries` now builds `UID SEARCH TEXT ...` criteria before fetching, and no longer filters `q` after fetching summaries.
  Verification: `TestMessageSummariesUseServerSideSearchBeforeLimit`; `go test ./...`.

- [x] Add explicit search scope handling for current mailbox vs current account.
  Evidence: `GET /api/accounts/{accountID}/mailboxes/{mailboxID}/messages?q=...&scope=mailbox|account` supports mailbox scope and current-account selectable mailbox search without cross-account unified search.
  Verification: `TestMessageSummariesAccountScopeSearchesSelectableMailboxes`; `npm run typecheck`.

- [x] Harden IMAP search query quoting and non-ASCII search behavior.
  Evidence: `searchCriteria` trims empty queries to `ALL`, quotes spaces/quotes/parentheses with `quoteIMAPString`, and uses `CHARSET UTF-8` for non-ASCII queries.
  Verification: `TestSearchCriteriaQuotesSpecialCharactersAndNonASCII`; `go test ./...`.

- [x] Decide and implement unread count source for accounts and mailboxes.
  Evidence: `handleAccounts` applies live IMAP `STATUS mailbox (UNSEEN)` counts and `accountForSession` sums mailbox unread counts into the account unread total.
  Verification: `TestAccountsPopulateUnreadCountsFromIMAPStatus`; `go test ./...`.

- [x] Add stronger session-expiry API behavior tests.
  Evidence: route-level expired signed session token test pins generic `401` behavior.
  Verification: `TestProtectedRoutesRejectExpiredSession`; `go test ./...`.

- [x] Add backend search performance guardrails before broad mailbox search.
  Evidence: server-side IMAP search delegates matching to IMAP, fetches only matching UID sets, caps per-mailbox and final account search summaries at `messageSummaryLimit`.
  Verification: `TestMessageSummariesUseServerSideSearchBeforeLimit`, `TestMessageSummariesAccountScopeSearchesSelectableMailboxes`; `go test ./...`.

## Frontend Gaps

- [x] Introduce a tracked search scope control once backend scope exists.
  Evidence: desktop `MessageList` and mobile `MobileInbox` expose current-mailbox/current-account controls and `App` sends `scope=mailbox|account` with search queries.
  Verification: `npm run typecheck`.

- [x] Replace non-virtualized message rendering with a scalable list strategy or document a deliberate deferral.
  Evidence: mobile product rendering no longer caps results with `messages.slice(0, 8)`.
  Deferred: true virtualization remains deferred because the plan recommends a new dependency such as `@tanstack/react-virtual`, and dependency additions require approval.
  Verification: `npm run typecheck`.

- [x] Add account/mailbox state restoration policy.
  Evidence: `App` persists active account, per-account mailbox, selected message, and search scope in `localStorage` under `joomail:mail-state`.
  Deferred: scroll-position restoration remains deferred until list virtualization or a stable scroll container policy is implemented.
  Verification: `npm run typecheck`.

- [x] Complete compose overlay controls from the plan or explicitly defer each missing control.
  Evidence: `ComposePanel` now supports desktop minimize/restore and expand/collapse controls.
  Deferred: Draft persistence and "save to Drafts then close" remain deferred because current phase excludes persistence and no backend Drafts save API exists.
  Verification: `npm run typecheck`.

- [x] Replace hard-coded attachment total size in the reading pane.
  Evidence: `ReadingPane` computes aggregate attachment size from backend-provided attachment size strings and omits the aggregate when parsing is not possible.
  Verification: `npm run typecheck`.

- [x] Add real quoted-content handling or mark as deferred.
  Evidence: `ReadingPane` detects parsed plain-text quote starts (`>` and `On ... wrote:`), hides quoted paragraphs by default, and expands only when quoted content exists.
  Verification: `npm run typecheck`.

## QA And Documentation Gaps

- [x] Run and record visual QA screenshots for all documented QA routes.
  Evidence: `docs/qa-ui-states.md` now includes a QA results log section for date, viewport coverage, screenshot location, and blockers.
  Deferred: actual screenshot capture for this batch is not recorded because the current workspace has no browser automation dependency and adding one requires approval; use the documented manual/browser-agent flow before release review.
  Verification: docs review; no code verification required.

- [x] Add a recurring source-vs-docs audit entry after each completed development batch.
  Evidence: this checklist was updated alongside backend/frontend changes with evidence, deferrals, and verification notes.
  Verification: `git diff -- docs/development-checklist.md`.

- [x] Keep README API list synchronized with backend routes.
  Evidence: README endpoint list still matches `Server.routes()` and documents the existing message-list `q` and `scope=mailbox|account` query options.
  Verification: compared `internal/httpapi/server.go` routes against README endpoint list.

## Explicit Non-Goals To Keep Out

These are planned exclusions, not missing work:

- Do not implement unified inbox unless the product plan changes.
- Do not implement conversation threading unless the product plan changes.
- Do not implement labels/tags, rules, scheduled send, undo send, snooze, contacts, calendar integration, account-wide unified search, list sorting options, comfort-density mode, mobile swipe gestures, advanced rich-text formatting, multiple compose windows, or dark-mode toggle UI without a separate product decision.
